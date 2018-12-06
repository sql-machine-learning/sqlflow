package sql

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/wangkuiyi/sqlfs"
)

func run(slct string, cfg *mysql.Config) error {
	r, e := newParser().Parse(slct)
	if e != nil {
		return e
	}
	if r.train {
		cwd, e := ioutil.TempDir("/tmp", "sqlflow-training")
		if e != nil {
			return e
		}
		defer os.RemoveAll(cwd)

		fts, e := verify(r, cfg)
		if e != nil {
			return e
		}

		if e := train(r, fts, cfg, cwd); e != nil {
			return e
		}
		m := &model{r, slct}
		if e := m.save(cfg, cwd); e != nil {
			return e
		}
	} else {
		inferParsed := r

		cwd, e := ioutil.TempDir("/tmp", "sqlflow-predicting")
		if e != nil {
			return e
		}
		defer os.RemoveAll(cwd)

		m := &model{parseResult: inferParsed}
		if e := m.load(cfg, cwd); e != nil {
			return e
		}

		trainParsed, e := newParser().Parse(m.TrainSelect)
		if e != nil {
			return e
		}

		if e := verifyColumnNameAndType(trainParsed, inferParsed, cfg); e != nil {
			return e
		}

		if e := createPredictionTable(trainParsed, inferParsed, cfg); e != nil {
			return e
		}

		if e := infer(trainParsed, inferParsed, cfg, cwd); e != nil {
			return e
		}
	}

	return nil
}

func train(pr *extendedSelect, fts fieldTypes, cfg *mysql.Config, cwd string) (e error) {
	var program bytes.Buffer
	if e := generateTFProgram(&program, pr, fts, cfg); e != nil {
		return e
	}

	cmd := tensorflowCmd(cwd)
	cmd.Stdin = bytes.NewReader(program.Bytes())
	o, e := cmd.CombinedOutput()
	if e != nil {
		return e
	}
	if !strings.Contains(string(o), "Done training") {
		return fmt.Errorf(string(o) + "\nTraining failed")
	}

	return nil
}

type model struct {
	parseResult *extendedSelect // private member will not be gob-encoded.
	TrainSelect string
}

func (m *model) save(cfg *mysql.Config, cwd string) (e error) {
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		return e
	}
	defer db.Close()

	sqlfn := fmt.Sprintf("sqlflow_models.%s", m.parseResult.save)
	sqlf, e := sqlfs.Create(db, sqlfn)
	if e != nil {
		return fmt.Errorf("Cannot create sqlfs file %s: %v", sqlfn, e)
	}
	defer func() { sqlf.Close() }()

	if e := gob.NewEncoder(sqlf).Encode(m); e != nil {
		return fmt.Errorf("model.save: gob-encoding model failed: %v", e)
	}

	cmd := exec.Command("tar", "czf", "-", "-C", cwd, ".")
	cmd.Stdout = sqlf
	return cmd.Run()
}

func (m *model) load(cfg *mysql.Config, cwd string) (e error) {
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		return e
	}
	defer db.Close()

	sqlfn := fmt.Sprintf("sqlflow_models.%s", m.parseResult.model)
	sqlf, e := sqlfs.Open(db, sqlfn)
	if e != nil {
		return fmt.Errorf("Cannot open sqlfs file %s: %v", sqlfn, e)
	}
	defer func() { sqlf.Close() }()

	// FIXME(tonyyang-svail): directly decoding from sqlf will cause out of bond
	// error, but it works fine if we loaded the whole chunk to the bytes.Buffer
	// then decode from there. More details at
	// https://github.com/wangkuiyi/sqlflow/issues/122
	var buf bytes.Buffer
	bs, e := ioutil.ReadAll(sqlf)
	if e != nil {
		return e
	}
	buf.Write(bs)
	if e := gob.NewDecoder(&buf).Decode(m); e != nil {
		return fmt.Errorf("model.load: gob-decoding model failed: %v", e)
	}

	cmd := exec.Command("tar", "xzf", "-", "-C", cwd)
	cmd.Stdin = &buf
	return cmd.Run()
}

// Create prediction table with appropriate column type.
// If prediction table already exists, it will be overwritten.
func createPredictionTable(trainParsed, inferParsed *extendedSelect, cfg *mysql.Config) (e error) {
	if len(strings.Split(inferParsed.into, ".")) != 3 {
		return fmt.Errorf("invalid inferParsed.into %s. should be DBName.TableName.ColumnName", inferParsed.into)
	}
	tableName := strings.Join(strings.Split(inferParsed.into, ".")[:2], ".")
	columnName := strings.Split(inferParsed.into, ".")[2]

	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		return fmt.Errorf("verify cannot connect to MySQL: %q", e)
	}
	defer db.Close()

	dropStmt := fmt.Sprintf("drop table if exists %s;", tableName)
	if _, e := db.Query(dropStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", dropStmt, e)
	}

	fts, e := verify(trainParsed, cfg)
	if e != nil {
		return e
	}
	tpy, _ := fts.get(trainParsed.label)

	createStmt := fmt.Sprintf("create table %s (%s %s);", tableName, columnName, tpy)
	if _, e := db.Query(createStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", createStmt, e)
	}

	return nil
}

func infer(trainParsed, inferParsed *extendedSelect, cfg *mysql.Config, cwd string) (e error) {
	return fmt.Errorf("infer not implemented")
}
