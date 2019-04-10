package sql

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os/exec"

	"github.com/sql-machine-learning/sqlflow/sqlfs"
)

type model struct {
	workDir     string // We don't expose and gob workDir; instead we tar it.
	TrainSelect string
}

// save creates a sqlfs table if it doesn't yet exist, and writes the
// train select statement into the table, followed by the tar-gzipped
// SQLFlow working directory, which contains the TensorFlow working
// directory and the trained TenosrFlow model.
func (m *model) save(db *DB, table string) (e error) {
	sqlf, e := sqlfs.Create(db.DB, table)
	if e != nil {
		return fmt.Errorf("Cannot create sqlfs file %s: %v", table, e)
	}
	defer sqlf.Close()

	// Use a bytes.Buffer as the gob message container to separate
	// the message from the following tarball.
	var buf bytes.Buffer
	if e := gob.NewEncoder(&buf).Encode(m); e != nil {
		return fmt.Errorf("model.save: gob-encoding model failed: %v", e)
	}
	if _, e := buf.WriteTo(sqlf); e != nil {
		return fmt.Errorf("model.save: write the buffer failed: %v", e)
	}

	cmd := exec.Command("tar", "czf", "-", "-C", m.workDir, ".")
	cmd.Stdout = sqlf
	return cmd.Run()
}

// load reads from the given sqlfs table for the train select
// statement, and untar the SQLFlow working directory, which contains
// the TenosrFlow model, into directory cwd.
func load(db *DB, table, cwd string) (m *model, e error) {
	sqlf, e := sqlfs.Open(db.DB, table)
	if e != nil {
		return nil, fmt.Errorf("Cannot open sqlfs file %s: %v", table, e)
	}
	defer sqlf.Close()

	var buf bytes.Buffer
	if _, e := buf.ReadFrom(sqlf); e != nil {
		return nil, e
	}
	m = &model{}
	if e := gob.NewDecoder(&buf).Decode(m); e != nil {
		return nil, fmt.Errorf("gob-decoding train select failed: %v", e)
	}

	cmd := exec.Command("tar", "xzf", "-", "-C", cwd)
	cmd.Stdin = &buf
	return m, cmd.Run()
}
