package sql

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Run executes a SQL query and returns a stream of row or message
func Run(slct string, db *sql.DB) *PipeReader {
	slctUpper := strings.ToUpper(slct)
	if strings.Contains(slctUpper, "TRAIN") || strings.Contains(slctUpper, "PREDICT") {
		pr, e := newParser().Parse(slct)
		if e == nil && pr.extended {
			// TODO(weiguo): mentioned in issue(abstract mysql specific code in sqlflow)
			dbCfg := &mysql.Config{
				User:   "root",
				Passwd: "root",
				Addr:   "localhost:3306",
			}
			return runExtendedSQL(slct, db, dbCfg, pr)
		}
	}
	return runStandardSQL(slct, db)
}

// TODO(weiguo): isQuery is a hacky way to decide which API to call:
// https://golang.org/pkg/database/sql/#DB.Exec .
// We will need to extend our parser to be a full SQL parser in the future.
func isQuery(slct string) bool {
	s := strings.ToUpper(strings.TrimSpace(slct))
	has := strings.Contains
	if strings.HasPrefix(s, "SELECT") && !has(s, "INTO") {
		return true
	}
	if strings.HasPrefix(s, "SHOW") && (has(s, "CREATE") || has(s, "DATABASES") || has(s, "TABLES")) {
		return true
	}
	if strings.HasPrefix(s, "DESCRIBE") {
		return true
	}
	return false
}

func runStandardSQL(slct string, db *sql.DB) *PipeReader {
	if isQuery(slct) {
		return runQuery(slct, db)
	}
	return runExec(slct, db)
}

// FIXME(tony): how to deal with large tables?
// TODO(tony): test on null table elements
func runQuery(slct string, db *sql.DB) *PipeReader {
	rd, wr := Pipe()
	go func() {
		defer wr.Close()

		re, se := func() (error, error) {
			startAt := time.Now()
			log.Infof("Starting runStanrardSQL:%s", slct)

			rows, err := db.Query(slct)
			if err != nil {
				return fmt.Errorf("runQuery failed: %v", err), nil
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return fmt.Errorf("failed to get columns: %v", err), nil
			}

			header := make(map[string]interface{})
			header["columnNames"] = cols
			if e := wr.Write(header); e != nil {
				return nil, e
			}

			// Since we don't know the table schema in advance, need to
			// create an slice of empty interface and adds column type
			// at runtime
			count := len(cols)
			values := make([]interface{}, count)
			columnTypes, err := rows.ColumnTypes()
			if err != nil {
				return fmt.Errorf("failed to get columnTypes: %v", err), nil
			}
			for i, ct := range columnTypes {
				v, e := createByType(ct.ScanType())
				if e != nil {
					return e, nil
				}
				values[i] = v
			}

			for rows.Next() {
				if err := rows.Scan(values...); err != nil {
					return err, nil
				}

				row := make([]interface{}, count)
				for i, val := range values {
					v, e := parseVal(val)
					if e != nil {
						return e, nil
					}
					row[i] = v
				}
				if e := wr.Write(row); e != nil {
					return nil, e
				}
			}
			log.Infof("runQuery finished, elapsed: %v", time.Since(startAt))
			return nil, nil
		}()
		if re != nil {
			log.Errorf("runQuery error:%v", re)
			se = wr.Write(re)
		}
		if se != nil {
			log.Errorf("runQuery error:%v", se)
		}
	}()
	return rd
}

func runExec(slct string, db *sql.DB) *PipeReader {
	rd, wr := Pipe()
	go func() {
		defer wr.Close()

		re, se := func() (error, error) {
			startAt := time.Now()
			log.Infof("Starting runStanrardSQL1:%s", slct)

			res, e := db.Exec(slct)
			if e != nil {
				return fmt.Errorf("runExec failed: %v", e), nil
			}
			affected, e := res.RowsAffected()
			if e != nil {
				return fmt.Errorf("failed to get affected row number: %v", e), nil
			}
			var msg string
			if affected > 1 {
				msg = fmt.Sprintf("%d rows affected", affected)
			} else {
				msg = fmt.Sprintf("%d row affected", affected)
			}
			if e := wr.Write(msg); e != nil {
				return nil, e
			}
			log.Infof("runExec finished, elapsed: %v", time.Since(startAt))
			return nil, nil
		}()
		if re != nil {
			log.Errorf("runExec error:%v", re)
			se = wr.Write(re)
		}
		if se != nil {
			log.Errorf("runExec error:%v", se)
		}
	}()
	return rd
}

func runExtendedSQL(slct string, db *sql.DB, cfg *mysql.Config, pr *extendedSelect) *PipeReader {
	rd, wr := Pipe()
	go func() {
		defer wr.Close()

		re, se := func() (error, error) {
			startAt := time.Now()
			log.Infof("Starting runExtendedSQL:%s", slct)

			// NOTE: the temporary directory must be in a host directory
			// which can be mounted to Docker containers.  If I don't
			// specify the "/tmp" prefix, ioutil.TempDir would by default
			// generate a directory in /private/tmp for macOS, which
			// cannot be mounted by Docker into the container.  For more
			// detailed, please refer to
			// https://docs.docker.com/docker-for-mac/osxfs/#namespaces.
			cwd, e := ioutil.TempDir("/tmp", "sqlflow")
			if e != nil {
				return e, nil
			}
			defer os.RemoveAll(cwd)

			if pr.train {
				for l := range train(pr, slct, db, cfg, cwd) {
					if e := wr.Write(l); e != nil {
						return nil, e
					}
				}
			} else {
				for l := range pred(pr, db, cfg, cwd) {
					if e := wr.Write(l); e != nil {
						return nil, e
					}
				}
			}
			log.Infof("runExtendedSQL finished, elapsed:%v", time.Since(startAt))
			return nil, nil
		}()

		if re != nil {
			log.Errorf("runExtendedSQL error:%v", re)
			se = wr.Write(re)
		}
		if se != nil {
			log.Errorf("runExtendedSQL error:%v", se)
		}
	}()
	return rd
}

type logChanWriter struct {
	c chan interface{}

	m    sync.Mutex
	buf  bytes.Buffer
	prev string
}

func (cw *logChanWriter) Write(p []byte) (n int, err error) {
	// Both cmd.Stdout and cmd.Stderr are writing to cw
	cw.m.Lock()
	defer cw.m.Unlock()

	n, err = cw.buf.Write(p)
	if err != nil {
		return n, err
	}

	for {
		line, err := cw.buf.ReadString('\n')
		cw.prev = cw.prev + line

		// ReadString returns err != nil if and only if the returned Data
		// does not end in delim.
		if err != nil {
			break
		}

		cw.c <- cw.prev
		cw.prev = ""
	}

	return n, nil
}

func train(tr *extendedSelect, slct string, db *sql.DB, cfg *mysql.Config, cwd string) chan interface{} {
	c := make(chan interface{})

	go func() {
		defer close(c)

		err := func() error {
			fts, e := verify(tr, db)
			if e != nil {
				return e
			}

			var program bytes.Buffer
			if e := genTF(&program, tr, fts, cfg); e != nil {
				return e
			}

			cw := &logChanWriter{c: c}
			cmd := tensorflowCmd(cwd)
			cmd.Stdin = &program
			cmd.Stdout = cw
			cmd.Stderr = cw
			if e := cmd.Run(); e != nil {
				return fmt.Errorf("training failed %v", e)
			}

			m := model{workDir: cwd, TrainSelect: slct}
			return m.save(db, tr.save)
		}()

		if err != nil {
			c <- err
		}
	}()

	return c
}

func pred(pr *extendedSelect, db *sql.DB, cfg *mysql.Config, cwd string) chan interface{} {
	c := make(chan interface{})

	go func() {
		defer close(c)

		err := func() error {
			m, e := load(db, pr.model, cwd)
			if e != nil {
				return e
			}

			// Parse the training SELECT statement used to train
			// the model for the prediction.
			tr, e := newParser().Parse(m.TrainSelect)
			if e != nil {
				return e
			}

			if e := verifyColumnNameAndType(tr, pr, db); e != nil {
				return e
			}

			if e := createPredictionTable(tr, pr, db); e != nil {
				return e
			}

			pr.trainClause = tr.trainClause
			fts, e := verify(pr, db)

			var buf bytes.Buffer
			if e := genTF(&buf, pr, fts, cfg); e != nil {
				return e
			}

			cw := &logChanWriter{c: c}
			cmd := tensorflowCmd(cwd)
			cmd.Stdin = &buf
			cmd.Stdout = cw
			cmd.Stderr = cw

			return cmd.Run()
		}()

		if err != nil {
			c <- err
		}
	}()

	return c
}

// Create prediction table with appropriate column type.
// If prediction table already exists, it will be overwritten.
func createPredictionTable(trainParsed, predParsed *extendedSelect, db *sql.DB) error {
	if len(strings.Split(predParsed.into, ".")) != 3 {
		return fmt.Errorf("invalid predParsed.into %s. should be DBName.TableName.ColumnName", predParsed.into)
	}
	tableName := strings.Join(strings.Split(predParsed.into, ".")[:2], ".")
	columnName := strings.Split(predParsed.into, ".")[2]

	dropStmt := fmt.Sprintf("drop table if exists %s;", tableName)
	if _, e := db.Query(dropStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", dropStmt, e)
	}

	fts, e := verify(trainParsed, db)
	if e != nil {
		return e
	}

	var b bytes.Buffer
	fmt.Fprintf(&b, "create table %s (", tableName)
	for _, c := range trainParsed.columns {
		typ, ok := fts.get(c.val)
		if !ok {
			return fmt.Errorf("createPredictionTable: Cannot find type of field %s", c.val)
		}
		fmt.Fprintf(&b, "%s %s, ", c.val, typ)
	}
	tpy, _ := fts.get(trainParsed.label)
	fmt.Fprintf(&b, "%s %s);", columnName, tpy)

	createStmt := b.String()
	if _, e := db.Query(createStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", createStmt, e)
	}
	return nil
}
