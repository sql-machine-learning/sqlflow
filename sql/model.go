package sql

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"fmt"
	"os/exec"

	"github.com/wangkuiyi/sqlfs"
)

type model struct {
	parseResult *extendedSelect // private member will not be gob-encoded.
	TrainSelect string
}

func (m *model) save(db *sql.DB, cwd string) (e error) {
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

func load(model_name string, db *sql.DB, cwd string) (m *model, e error) {
	sqlfn := fmt.Sprintf("sqlflow_models.%s", model_name)
	sqlf, e := sqlfs.Open(db, sqlfn)
	if e != nil {
		return nil, fmt.Errorf("Cannot open sqlfs file %s: %v", sqlfn, e)
	}
	defer func() { sqlf.Close() }()

	// FIXME(tonyyang-svail): directly decoding from sqlf will cause out of bond
	// error, but it works fine if we loaded the whole chunk to the bytes.Buffer
	// then decode from there. More details at
	// https://github.com/wangkuiyi/sqlflow/issues/122
	var buf bytes.Buffer
	_, e = buf.ReadFrom(sqlf)
	if e != nil {
		return nil, e
	}

	m = &model{}
	if e := gob.NewDecoder(&buf).Decode(m); e != nil {
		return nil, fmt.Errorf("model.load: gob-decoding model failed: %v", e)
	}

	cmd := exec.Command("tar", "xzf", "-", "-C", cwd)
	cmd.Stdin = &buf
	return m, cmd.Run()
}
