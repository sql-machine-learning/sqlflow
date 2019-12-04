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
	"path"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/proto"

	"sqlflow.org/sqlflow/pkg/sql/ir"

	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sqlfs"
)

const modelZooDB = "sqlflow"
const modelZooTable = "sqlflow.trained_models"

type model struct {
	workDir     string // We don't expose and gob workDir; instead we tar it.
	TrainSelect string
}

func (m *model) save(modelURI string, trainIR *ir.TrainClause, session *pb.Session) error {
	if strings.Contains(modelURI, "://") {
		uriParts := strings.Split(modelURI, "://")
		if len(uriParts) == 2 {
			// oss:// or file://
			if uriParts[0] == "file" {
				dir, file := path.Split(uriParts[1])
				return m.saveTar(dir, file)
			} else if uriParts[0] == "oss" {
				return fmt.Errorf("save model to oss is not supported now")
			}
		} else {
			return fmt.Errorf("error modelURI format: %s", modelURI)
		}
	}
	db, err := NewDB(session.DbConnStr)
	if err != nil {
		return err
	}
	if err := m.saveDB(db, modelURI, session); err != nil {
		return err
	}
	// TODO(typhoonzero): support hive, maxcompute saving model zoo metas.
	if db.driverName == "mysql" {
		// Save model metas in model zoo table
		if err := createModelZooTable(db); err != nil {
			return err
		}
		return addTrainedModelsRecord(db, trainIR, modelURI, session)
	}
	return nil
}

func load(modelURI, dst string, db *DB) (*model, error) {
	// FIXME(typhoonzero): unify arguments with save, use session,
	// so that can pass oss credentials too.
	if strings.Contains(modelURI, "://") {
		uriParts := strings.Split(modelURI, "://")
		if len(uriParts) == 2 {
			// oss:// or file://
			if uriParts[0] == "file" {
				dir, file := path.Split(uriParts[1])
				return loadTar(dir, dst, file)
			} else if uriParts[0] == "oss" {
				return nil, fmt.Errorf("save model to oss is not supported now")
			}
		} else {
			return nil, fmt.Errorf("error modelURI format: %s", modelURI)
		}
	}
	return loadDB(db, modelURI, dst)
}

// saveDB creates a sqlfs table if it doesn't yet exist, and writes the
// train select statement into the table, followed by the tar-gzipped
// SQLFlow working directory, which contains the TensorFlow working
// directory and the trained TensorFlow model.
func (m *model) saveDB(db *DB, table string, session *pb.Session) (e error) {
	sqlf, e := sqlfs.Create(db.DB, db.driverName, table, session)
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

	if e := sqlf.Close(); e != nil {
		return fmt.Errorf("close sqlfs error: %v", e)
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
		return nil, fmt.Errorf("load tar file(%s) failed: %v", tarFile, e)
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
// the TensorFlow model, into directory cwd.
func loadDB(db *DB, table, cwd string) (m *model, e error) {
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

// createModelZooTable create the table "sqlflow.trained_models" to save model
// metas the saved model URI.
func createModelZooTable(db *DB) error {
	createDBSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", modelZooDB)
	_, err := db.Exec(createDBSQL)
	if err != nil {
		return err
	}
	// schema design: https://github.com/sql-machine-learning/sqlflow/blob/a98218ef8bee57e2a45357d7ee5721e1c6dfeb35/doc/design/model_zoo.md#model-zoo-data-schema
	// NOTE(typhoonzero): submitter program size may exceed TEXT size(64KB)
	// NOTE(typhoonzero): train_ir_pb contains all information how the submitter program is generated, so not saving submitter program now
	// NOTE(typhoonzero): model_uri can be:
	//    1. file:///path/to/model/dir
	//    2. db.table
	//    3. oss://path/to/oss
	createTableSQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
model_id VARCHAR(255),
author VARCHAR(255),
model_image VARCHAR(255),
model_def VARCHAR(255),
train_ir_pb TEXT,
model_uri VARCHAR(255)
);`, modelZooTable)
	_, err = db.Exec(createTableSQL)
	return err
}

func getTrainedModelParts(into string) (string, string, error) {
	if strings.ContainsRune(into, '.') {
		parts := strings.Split(into, ".")
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
		return "", "", fmt.Errorf("error INTO format, should be like [creator.]modelID, but got %s", into)
	}
	return "", into, nil
}

func dbStringEscape(src string) string {
	ret := strings.ReplaceAll(src, "\"", "\\\"")
	return strings.ReplaceAll(ret, "'", "\\'")
}

func addTrainedModelsRecord(db *DB, trainIR *ir.TrainClause, modelURI string, sess *pb.Session) error {
	// NOTE(typhoonzero): creator can be empty, if so, the model file is saved into current database
	// FIXME(typhoonzero): or maybe the into format should be like "creator/modelID"
	creator, modelID, err := getTrainedModelParts(trainIR.Into)
	if err != nil {
		return err
	}

	q := fmt.Sprintf("SELECT * FROM %s WHERE model_id='%s'", modelZooTable, modelID)
	res, err := db.Query(q)
	if err != nil {
		return err
	}
	defer res.Close()
	isInsert := false
	if !res.Next() {
		isInsert = true
	}
	var sql string
	irproto, err := ir.TrainIRToProto(trainIR, sess)
	if err != nil {
		return err
	}
	irprotoText := proto.MarshalTextString(irproto)
	if isInsert {
		sql = fmt.Sprintf(`INSERT INTO %s
(model_id, author, model_image, model_def, train_ir_pb, model_uri)
VALUES ("%s", "%s", "%s", "%s", "%s", "%s")`,
			modelZooTable, modelID, creator, trainIR.ModelImage, trainIR.Estimator, dbStringEscape(irprotoText), modelURI)
	} else {
		sql = fmt.Sprintf(`UPDATE %s SET
author="%s",
model_image="%s",
model_def="%s",
train_ir_pb="%s",
model_uri="%s"
WHERE model_id="%s"`, modelZooTable, creator, trainIR.ModelImage, trainIR.Estimator, dbStringEscape(irprotoText), modelURI, modelID)
	}
	_, err = db.Exec(sql)
	return err
}
