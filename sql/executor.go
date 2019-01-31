package sql

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Row contains a database row and an error, if not nil, in retrieving the row
type Row struct {
	row []interface{}
	err error
}

// Run executes a SQLFlow statements, either standard or extended
// - Standard SQL statements like `USE database` returns a success message.
// - Standard SQL statements like `SELECT ...` returns a table in addition
// to the status, and the table might be big.
// - Extended SQL statement like `SELECT ... TRAIN/PREDICT ...` messages
// which indicate the training/predicting progress
func Run(slct string, db *sql.DB, cfg *mysql.Config) (string, error) {
	slctUpper := strings.ToUpper(slct)
	if strings.Contains(slctUpper, "TRAIN") || strings.Contains(slctUpper, "PREDICT") {
		pr, e := newParser().Parse(slct)
		if e == nil && pr.extended {
			if e = runExtendedSQL(slct, db, cfg, pr); e != nil {
				log.Errorf("runExtendedSQL error:%v", e)
				return "", e
			}
			return "Job success", nil
		}
	}

	var buf bytes.Buffer
	c := runStandardSQL(slct, db)
	for r := range c {
		if r.err != nil {
			return "", r.err
		}
		fmt.Fprintf(&buf, "%v\n", r.row)
	}
	return string(buf.Bytes()), nil
}

// FIXME(tony): how to deal with large tables?
// TODO(tony): test on null table elements
func runStandardSQL(slct string, db *sql.DB) chan Row {
	c := make(chan Row)

	go func() {
		defer close(c)

		err := func() error {
			startAt := time.Now()
			log.Infof("Starting runStanrardSQL:%s", slct)

			rows, err := db.Query(slct)
			defer rows.Close()
			if err != nil {
				return fmt.Errorf("runStandardSQL failed: %v", err)
			}

			cols, err := rows.Columns()
			if err != nil {
				return fmt.Errorf("failed to get columns: %v", err)
			}

			// Since we don't know the table schema in advance, need to
			// create an slice of empty interface and adds column type
			// at runtime
			count := len(cols)
			values := make([]interface{}, count)
			columnTypes, err := rows.ColumnTypes()
			if err != nil {
				return fmt.Errorf("failed to get columnTypes: %v", err)
			}
			// FIXME(tony): support more column types: https://golang.org/pkg/database/sql/#Rows.Scan
			for i, ct := range columnTypes {
				switch ct.ScanType() {
				case reflect.TypeOf(sql.NullBool{}):
					values[i] = new(bool)
				case reflect.TypeOf(sql.NullInt64{}):
					values[i] = new(int64)
				case reflect.TypeOf(sql.NullFloat64{}):
					values[i] = new(float64)
				case reflect.TypeOf(sql.NullString{}):
					values[i] = new(string)
				default:
					return fmt.Errorf("unrecognized column scan type %v", ct.ScanType())
				}
			}

			for rows.Next() {
				err = rows.Scan(values...)
				if err != nil {
					return err
				}

				row := make([]interface{}, count)
				for i, val := range values {
					switch v := val.(type) {
					case *bool:
						row[i] = *v
					case *int64:
						row[i] = *v
					case *float64:
						row[i] = *v
					case *string:
						row[i] = *v
					default:
						return fmt.Errorf("unrecogized type %v", v)
					}
				}
				c <- Row{row, nil}
			}

			log.Infof("runStandardSQL finished, elapsed: %v", time.Now().Sub(startAt))
			return nil
		}()

		if err != nil {
			c <- Row{nil, err}
		}
	}()

	return c
}

func runExtendedSQL(slct string, db *sql.DB, cfg *mysql.Config, pr *extendedSelect) error {
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
		return e
	}
	defer os.RemoveAll(cwd)

	if pr.train {
		e = train(pr, slct, db, cfg, cwd)
	} else {
		e = pred(pr, db, cfg, cwd)
	}
	log.Infof("runExtendedSQL finished, elapsed:%v", time.Now().Sub(startAt))
	return e
}

func train(tr *extendedSelect, slct string, db *sql.DB, cfg *mysql.Config, cwd string) error {
	fts, e := verify(tr, db)
	if e != nil {
		return e
	}

	var program bytes.Buffer
	if e := genTF(&program, tr, fts, cfg); e != nil {
		return e
	}

	cmd := tensorflowCmd(cwd)
	cmd.Stdin = &program
	o, e := cmd.CombinedOutput()
	if e != nil || !strings.Contains(string(o), "Done training") {
		return fmt.Errorf("Training failed %v: \n%s", e, o)
	}

	m := model{workDir: cwd, TrainSelect: slct}
	return m.save(db, tr.save)
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

func pred(pr *extendedSelect, db *sql.DB, cfg *mysql.Config, cwd string) error {
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

	cmd := tensorflowCmd(cwd)
	cmd.Stdin = &buf
	o, e := cmd.CombinedOutput()
	if e != nil || !strings.Contains(string(o), "Done predicting") {
		return fmt.Errorf("Prediction failed %v: \n%s", e, o)
	}
	return nil
}
