// Copyright 2019 The SQLFlow Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sql

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
	sqlf, e := sqlfs.Create(db.DB, db.driverName, table)
	if e != nil {
		return fmt.Errorf("cannot create sqlfs file %s: %v", table, e)
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
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if e := cmd.Run(); e != nil {
		return fmt.Errorf("tar stderr: %v\ntar cmd %v", errBuf.String(), e)
	}
	return nil
}

func (m *model) saveTar(modelDir, save string) (e error) {
	gobFile := filepath.Join(m.workDir, save+".gob")
	if e := writeGob(gobFile, m); e != nil {
		return e
	}
	modelFile := filepath.Join(modelDir, save+".tar.gz")
	cmd := exec.Command("tar", "czf", modelFile, "-C", m.workDir, ".")
	return cmd.Run()
}

func loadTar(modelDir, cwd, save string) (m *model, e error) {
	tarFile := filepath.Join(modelDir, save+".tar.gz")
	cmd := exec.Command("tar", "zxf", tarFile, "-C", cwd)
	if e = cmd.Run(); e != nil {
		return nil, fmt.Errorf("load tar file failed: %v", e)
	}
	gobFile := filepath.Join(cwd, save+".gob")
	m = &model{}
	if e = readGob(gobFile, m); e != nil {
		return nil, e
	}
	return m, nil
}

// load reads from the given sqlfs table for the train select
// statement, and untar the SQLFlow working directory, which contains
// the TenosrFlow model, into directory cwd.
func load(db *DB, table, cwd string) (m *model, e error) {
	sqlf, e := sqlfs.Open(db.DB, table)
	if e != nil {
		return nil, fmt.Errorf("cannot open sqlfs file %s: %v", table, e)
	}
	defer sqlf.Close()

	var buf bytes.Buffer
	if _, e := buf.ReadFrom(sqlf); e != nil {
		return nil, fmt.Errorf("buf.ReadFrom %v", e)
	}
	m = &model{}
	if e := gob.NewDecoder(&buf).Decode(m); e != nil {
		return nil, fmt.Errorf("gob-decoding train select failed: %v", e)
	}

	cmd := exec.Command("tar", "xzf", "-", "-C", cwd)
	cmd.Stdin = &buf
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tar %v", string(output))
	}
	return m, nil
}

func writeGob(filePath string, object interface{}) error {
	file, e := os.Create(filePath)
	if e != nil {
		return fmt.Errorf("create gob file :%s, error: %v", filePath, e)
	}
	defer file.Close()
	if e := gob.NewEncoder(file).Encode(object); e != nil {
		return fmt.Errorf("model.save: gob-encoding model failed: %v", e)
	}
	return nil
}

func readGob(filePath string, object interface{}) error {
	file, e := os.Open(filePath)
	if e != nil {
		return fmt.Errorf("model.load: gob-decoding model failed: %v", e)
	}
	defer file.Close()
	if e := gob.NewDecoder(file).Decode(object); e != nil {
		return fmt.Errorf("model.load: gob-decoding model failed: %v", e)
	}
	return nil
}
