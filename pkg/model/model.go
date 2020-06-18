// Copyright 2020 The SQLFlow Authors. All rights reserved.
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

package model

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"google.golang.org/grpc"
	"sqlflow.org/sqlflow/pkg/database"

	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sqlfs"
)

const modelZooDB = "sqlflow"
const modelZooTable = "sqlflow.trained_models"

// Model represent a trained model, which could be saved to a filesystem or sqlfs.
type Model struct {
	workDir     string // We don't expose and gob workDir; instead we tar it.
	TrainSelect string // TrainSelect is gob-encoded during I/O.
}

// New an empty model.
func New(cwd, trainSelect string) *Model {
	return &Model{
		workDir:     cwd,
		TrainSelect: trainSelect}
}

// Save all files in workDir as a tarball to a filesystem or sqlfs.
func (m *Model) Save(modelURI string, session *pb.Session) error {
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
	db, err := database.OpenAndConnectDB(session.DbConnStr)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := m.saveDB(db, modelURI, session); err != nil {
		return err
	}
	return nil
}

// Load untar a saved model to a directory on the local filesystem.
// When dst=="", we do not untar model data, just extract the model meta
func Load(modelURI, dst string, db *database.DB) (*Model, error) {
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
				return nil, fmt.Errorf("load model from oss is not supported now")
			}
		} else {
			return nil, fmt.Errorf("error modelURI format: %s", modelURI)
		}
	} else if strings.Contains(modelURI, "/") {
		// general model zoo urls like some-domain.com:port/model_name:tag
		// download traind model and extract to dst
		return loadModelFromZoo(modelURI, dst)
	}
	return loadDBAndUntar(db, modelURI, dst)
}

func downloadModelToBuf(modelZooServerAddr, modelName, modelTag string) (bytes.Buffer, error) {
	var buf bytes.Buffer
	// TODO(typhoonzero): use SSL connection
	conn, err := grpc.Dial(modelZooServerAddr, grpc.WithInsecure())
	if err != nil {
		return buf, err
	}
	defer conn.Close()
	modelZooClient := pb.NewModelZooServerClient(conn)
	stream, err := modelZooClient.DownloadModel(context.Background(), &pb.ReleaseModelRequest{
		Name: modelName,
		Tag:  modelTag,
	})
	if err != nil {
		return buf, err
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return buf, err
		}
		_, err = buf.Write(resp.ContentTar)
		if err != nil {
			return buf, err
		}
	}
	return buf, nil
}

func loadModelFromZoo(modelURI, dst string) (*Model, error) {
	// modelURI is formated like some-domain.com:port/model_name:tag
	uriParts := strings.Split(modelURI, "/")
	// previous checks asserted that uriParts must have two parts
	modelZooServerAddr := uriParts[0]
	modelNameAndTag := strings.Join(uriParts[1:], "/")
	parts := strings.Split(modelNameAndTag, ":")
	var modelName string
	var modelTag string
	if len(parts) == 1 {
		modelName = modelNameAndTag
		modelTag = "" // default use empty tag
	} else if len(parts) == 2 {
		modelName = parts[0]
		modelTag = parts[1]
	} else {
		return nil, fmt.Errorf("model name after USING must be like [model-zoo.com:port/]model_name[:tag]")
	}

	buf, err := downloadModelToBuf(modelZooServerAddr, modelName, modelTag)
	if err != nil {
		return nil, err
	}

	m := &Model{}
	if e := gob.NewDecoder(&buf).Decode(m); e != nil {
		return nil, fmt.Errorf("gob-decoding train select failed: %v", e)
	}

	if dst != "" { // empty is invalid param for tar -C
		if err := untarBuf(buf, dst); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// untarBuf extracts a tar.gz buffer from stdin and write the extracted files to dst
func untarBuf(buf bytes.Buffer, dst string) error {
	cmd := exec.Command("tar", "xzf", "-", "-C", dst)
	cmd.Stdin = &buf
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar %v", string(output))
	}
	return nil
}

// saveDB creates a sqlfs table if it doesn't yet exist, and writes the
// train select statement into the table, followed by the tar-gzipped
// SQLFlow working directory, which contains the TensorFlow working
// directory and the trained TensorFlow model.
func (m *Model) saveDB(db *database.DB, table string, session *pb.Session) (e error) {
	sqlf, e := sqlfs.Create(db.DB, db.DriverName, table, session)
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

func (m *Model) saveTar(modelDir, save string) (e error) {
	gobFile := filepath.Join(m.workDir, save+".gob")
	if e := writeGob(gobFile, m); e != nil {
		return e
	}
	modelFile := filepath.Join(modelDir, save+".tar.gz")
	cmd := exec.Command("tar", "czf", modelFile, "-C", m.workDir, ".")
	return cmd.Run()
}

func loadTar(modelDir, cwd, save string) (m *Model, e error) {
	tarFile := filepath.Join(modelDir, save+".tar.gz")
	cmd := exec.Command("tar", "zxf", tarFile, "-C", cwd)
	if e = cmd.Run(); e != nil {
		return nil, fmt.Errorf("load tar file(%s) failed: %v", tarFile, e)
	}
	gobFile := filepath.Join(cwd, save+".gob")
	m = &Model{}
	if e = readGob(gobFile, m); e != nil {
		return nil, e
	}
	return m, nil
}

// loadDBAndUntar reads from the given sqlfs table for the train select
// statement, and untar the SQLFlow working directory, which contains
// the TensorFlow model, into directory cwd if cwd is not "".
func loadDBAndUntar(db *database.DB, table, cwd string) (*Model, error) {
	buf, err := LoadToBuffer(db, table)
	if err != nil {
		return nil, err
	}
	model, err := DecodeModel(buf)
	if err != nil {
		return nil, err
	}
	// empty is invalid param for tar -C
	if cwd == "" {
		return model, nil
	}
	if err = untarBuf(*buf, cwd); err != nil {
		return nil, err
	}
	return model, nil
}

// DecodeModel decode model metadata from a byte buffer
func DecodeModel(buf *bytes.Buffer) (*Model, error) {
	m := &Model{}
	if e := gob.NewDecoder(buf).Decode(m); e != nil {
		return nil, fmt.Errorf("gob-decoding train select failed: %v", e)
	}
	return m, nil
}

// LoadToBuffer reads model data from database to a buffer
func LoadToBuffer(db *database.DB, table string) (buf *bytes.Buffer, e error) {
	sqlf, e := sqlfs.Open(db.DB, table)
	if e != nil {
		return nil, fmt.Errorf("cannot open sqlfs file %s: %v", table, e)
	}
	defer sqlf.Close()

	buf = &bytes.Buffer{}
	// FIXME(typhoonzero): ReadFrom may panic with ErrTooLarge
	// need to put the "Model" struct under extracted model files
	if _, e := buf.ReadFrom(sqlf); e != nil {
		return nil, fmt.Errorf("buf.ReadFrom %v", e)
	}

	return buf, nil
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
