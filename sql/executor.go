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

		return fmt.Errorf("not implemented")

		trainParsed, e := newParser().Parse(m.TrainSelect)
		if e != nil {
			return e
		}

		fts, e := verifyColumnTypes(trainParsed, inferParsed);
		if e != nil {
			return e
		}

		if e := preparePredictionTable(inferParsed, cfg); e != nil {
			return e
		}

		if e := infer(trainParsed, inferParsed, fts, cfg, cwd); e != nil {
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

func verifyColumnTypes(trainParsed, inferParsed *extendedSelect) (fts fieldTypes, e error) {
	return fieldTypes{}, fmt.Errorf("verifyColumnTypes not implemented")
}

func preparePredictionTable(pr *extendedSelect, cfg *mysql.Config) (e error) {
	return fmt.Errorf("preparePredictionTable not implemented")
}

func infer(trainParsed, inferParsed *extendedSelect, fts fieldTypes, cfg *mysql.Config, cwd string) (e error) {
	return fmt.Errorf("model.load not implemented")
}
