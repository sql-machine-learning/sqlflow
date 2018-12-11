package sql

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"fmt"
	"os/exec"

	"github.com/wangkuiyi/sqlflow/sqlfs"
)

// save creates a sqlfs table if it doesn't yet exist, and writes the
// train select statement into the table, followed by the tar-gzipped
// SQLFlow working directory, which contains the TensorFlow working
// directory and the trained TenosrFlow model.
func save(db *sql.DB, table, cwd, trainSlct string) (e error) {
	sqlfn := fmt.Sprintf("sqlflow_models.%s", table)
	sqlf, e := sqlfs.Create(db, sqlfn)
	if e != nil {
		return fmt.Errorf("Cannot create sqlfs file %s: %v", sqlfn, e)
	}
	defer sqlf.Close()

	if e := gob.NewEncoder(sqlf).Encode(trainSlct); e != nil {
		return fmt.Errorf("model.save: gob-encoding model failed: %v", e)
	}

	cmd := exec.Command("tar", "czf", "-", "-C", cwd, ".")
	cmd.Stdout = sqlf
	return cmd.Run()
}

// load reads from the given sqlfs table for the train select
// statement, and untar the SQLFlow working directory, which contains
// the TenosrFlow model, into directory cwd.
func load(db *sql.DB, table, cwd string) (trainSlct string, e error) {
	sqlfn := fmt.Sprintf("sqlflow_models.%s", table)
	sqlf, e := sqlfs.Open(db, sqlfn)
	if e != nil {
		return "", fmt.Errorf("Cannot open sqlfs file %s: %v", sqlfn, e)
	}
	defer sqlf.Close()

	// FIXME(tonyyang-svail): directly decoding from sqlf will cause out of bond
	// error, but it works fine if we loaded the whole chunk to the bytes.Buffer
	// then decode from there. More details at
	// https://github.com/wangkuiyi/sqlflow/issues/122
	var buf bytes.Buffer
	_, e = buf.ReadFrom(sqlf)
	if e != nil {
		return "", e
	}

	if e := gob.NewDecoder(&buf).Decode(&trainSlct); e != nil {
		return "", fmt.Errorf("gob-decoding train select failed: %v", e)
	}

	cmd := exec.Command("tar", "xzf", "-", "-C", cwd)
	cmd.Stdin = &buf
	return trainSlct, cmd.Run()
}
