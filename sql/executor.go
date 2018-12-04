package sql

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/wangkuiyi/sqlfs"
)

func run(slct string, cfg *mysql.Config) error {
	r, e := newParser().Parse(slct)
	if e != nil {
		return e
	}
	fts, e := verify(r, cfg)
	if e != nil {
		return e
	}

	if r.train {
		cwd, e := ioutil.TempDir("/tmp", "sqlflow-training")
		if e != nil {
			return e
		}
		defer os.RemoveAll(cwd)

		if e := train(r, fts, cfg, cwd); e != nil {
			return e
		}
		m := &model{r, slct}
		if e := m.save(cfg, cwd); e != nil {
			return e
		}
	} else {
		return fmt.Errorf("inference not implemented")
	}

	return nil
}

func train(pr *extendedSelect, fts fieldTypes, cfg *mysql.Config, cwd string) error {
	var program bytes.Buffer
	if e := generateTFProgram(&program, pr, fts, cfg); e != nil {
		return e
	}

	cmd := tensorflowCmd(cwd)
	cmd.Stdin = bytes.NewReader(program.Bytes())
	o, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	if !strings.Contains(string(o), "Done training") {
		return fmt.Errorf(string(o) + "\nTraining failed")
	}

	return nil
}

type model struct {
	parseResult *extendedSelect // private member will not be gob-encoded.
	Slct        string
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
	defer func() { e = sqlf.Close() }()

	if e := gob.NewEncoder(sqlf).Encode(m); e != nil {
		return fmt.Errorf("model.save: gob-encoding model failed: %v", e)
	}

	dir := filepath.Join(cwd, m.parseResult.save)
	cmd := exec.Command("tar", "Pczf", "-", dir)
	cmd.Stdout = sqlf
	return cmd.Run()
}
