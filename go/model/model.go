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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bitly/go-simplejson"
	"google.golang.org/grpc"
	"sqlflow.org/sqlflow/go/database"

	pb "sqlflow.org/sqlflow/go/proto"
	"sqlflow.org/sqlflow/go/sqlfs"
)

const (
	modelZooDB        = "sqlflow"
	modelZooTable     = "sqlflow.trained_models"
	modelMetaFileName = "model_meta.json"
)

const (
	// TENSORFLOW is the mode type that trained by TensorFlow.
	TENSORFLOW = iota
	// XGBOOST is the model type that trained by XGBoost models.
	XGBOOST
	// PAIML is the model type that trained by PAI machine learning algorithm toolkit
	PAIML
)

// Model represent a trained model, which could be saved to a filesystem or sqlfs.
type Model struct {
	workDir     string           // We don't expose and gob workDir; instead we tar it.
	TrainSelect string           // TrainSelect is gob-encoded during I/O.
	Meta        *simplejson.Json // Meta json object
}

// New an empty model.
func New(cwd, trainSelect string) *Model {
	return &Model{
		workDir:     cwd,
		TrainSelect: trainSelect}
}

// GetMetaAsString return specified metadata as string
func (m *Model) GetMetaAsString(key string) string {
	if m.Meta == nil {
		return ""
	}
	return m.Meta.Get(key).MustString()
}

// Save all files in workDir as a tarball to a filesystem or sqlfs.
func (m *Model) Save(modelURI string, session *pb.Session) error {
	if strings.Contains(modelURI, "://") {
		uriParts := strings.Split(modelURI, "://")
		if len(uriParts) == 2 {
			// oss:// or file://
			if uriParts[0] == "file" {
				dir, file := path.Split(uriParts[1])
				_, err := m.saveTar(dir, file)
				return err
			} else if uriParts[0] == "oss" {
				return fmt.Errorf("save model to oss is not supported now")
			}
		} else {
			return fmt.Errorf("error modelURI format: %s", modelURI)
		}
	}

	return m.saveDB(session.DbConnStr, modelURI, session)
}

// Load unzip a saved model to a directory on the local filesystem.
// When dst=="", we do not unzip model data, just extract the model meta
func Load(modelURI, dst string, db *database.DB) (*Model, error) {
	// FIXME(typhoonzero): unify arguments with save, use session,
	// so that can pass oss credentials too.
	if strings.Contains(modelURI, "://") {
		uriParts := strings.Split(modelURI, "://")
		if len(uriParts) == 2 {
			// oss:// or file://
			if uriParts[0] == "file" {
				dir, file := path.Split(uriParts[1])
				return loadTar(dir, file, dst)
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
	return loadModelFromDB(db, modelURI, dst)
}

func downloadModel(modelZooServerAddr,
	modelName, modelTag string, tmpFile *os.File) error {

	// TODO(typhoonzero): use SSL connection
	conn, err := grpc.Dial(modelZooServerAddr, grpc.WithInsecure())
	if err != nil {
		return err
	}
	defer conn.Close()
	modelZooClient := pb.NewModelZooServerClient(conn)
	stream, err := modelZooClient.DownloadModel(context.Background(),
		&pb.ReleaseModelRequest{
			Name: modelName,
			Tag:  modelTag,
		})
	if err != nil {
		return err
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		_, err = tmpFile.Write(resp.ContentTar)
		if err != nil {
			return err
		}
	}
	return nil
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

	tmpFile, err := ioutil.TempFile(os.TempDir(), "downloaded_model*.tar.gz")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())
	err = downloadModel(modelZooServerAddr, modelName, modelTag, tmpFile)
	if err != nil {
		return nil, err
	}

	return loadTar(path.Dir(tmpFile.Name()), path.Base(tmpFile.Name()), dst)
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
func (m *Model) saveDB(connStr, table string, session *pb.Session) (e error) {
	db, err := database.OpenAndConnectDB(connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	sqlf, e := sqlfs.Create(db, table, session)
	if e != nil {
		return fmt.Errorf("cannot create sqlfs file %s: %v", table, e)
	}
	defer sqlf.Close()

	// model and its metadata are both zipped into a tarball
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

// SaveDBExperimental save the model to database with metadata using the refactored format.
func (m *Model) SaveDBExperimental(connStr, table string, session *pb.Session) (e error) {
	db, err := database.OpenAndConnectDB(connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	sqlf, e := sqlfs.Create(db, table, session)
	if e != nil {
		return fmt.Errorf("cannot create sqlfs file %s: %v", table, e)
	}
	defer sqlf.Close()

	metaJSONStr, err := m.Meta.Encode()
	if err != nil {
		return err
	}
	metaLen := len(metaJSONStr)
	metaLenHex := fmt.Sprintf("0x%08x", metaLen)
	sqlf.Write([]byte(metaLenHex))
	sqlf.Write([]byte(metaJSONStr))

	// model and its metadata are both zipped into a tarball
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

func (m *Model) saveTar(modelDir, save string) (string, error) {
	save = strings.TrimSuffix(save, ".tar.gz")
	modelFile := filepath.Join(modelDir, save+".tar.gz")
	cmd := exec.Command("tar", "czf", modelFile, "-C", m.workDir, ".")
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return modelFile, nil
}

func loadTar(modelDir, save, dst string) (*Model, error) {
	save = strings.TrimSuffix(save, ".tar.gz")
	tarFile := filepath.Join(modelDir, save+".tar.gz")
	cmd := exec.Command("tar", "zxf", tarFile, "-C", dst)
	if e := cmd.Run(); e != nil {
		return nil, fmt.Errorf("unzip tar file %s failed", modelDir)
	}
	return loadMeta(path.Join(dst, modelMetaFileName))
}

// loadModelFromDB reads from the given sqlfs table for the train select
// statement, and unzip the SQLFlow working directory, which contains
// the TensorFlow model, into directory cwd if cwd is not "".
func loadModelFromDB(db *database.DB, table, cwd string) (*Model, error) {
	unzipModel := cwd != ""
	if cwd == "" {
		var err error
		if cwd, err = ioutil.TempDir("/tmp", "sqlflow_models"); err != nil {
			return nil, err
		}
		defer os.RemoveAll(cwd)
	}
	tarFile, err := DumpDBModel(db, table, cwd)
	if err != nil {
		return nil, err
	}
	if !unzipModel {
		return ExtractMetaFromTarball(tarFile, cwd)
	}
	return loadTar(path.Dir(tarFile), path.Base(tarFile), cwd)
}

// DumpDBModel dumps a model tarball from database to local
// file system and return the file name
func DumpDBModel(db *database.DB, table, cwd string) (string, error) {
	sqlf, err := sqlfs.Open(db.DB, table)
	if err != nil {
		return "", fmt.Errorf("Can't open sqlfs %s, %v", table, err)
	}
	defer sqlf.Close()
	fileName := filepath.Join(cwd, "model_dump.tar.gz")
	file, err := os.Create(fileName)
	if err != nil {
		return "", fmt.Errorf("Can't careate model file: %v", err)
	}
	if _, err = io.Copy(file, sqlf); err != nil {
		return "", fmt.Errorf("Can't dump model to local file")
	}
	return fileName, nil
}

// ExtractMetaFromTarball extract metadata from given tarball
// and return the metadata json string. This function do not
// unzip the whole model tarball
func ExtractMetaFromTarball(tarballName, cwd string) (*Model, error) {
	if !strings.HasSuffix(tarballName, ".tar.gz") {
		return nil, fmt.Errorf("given file should be a .tar.gz file")
	}
	cmd := exec.Command("tar", "xpf", tarballName, "-C", cwd, "./"+modelMetaFileName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("can't unzip model tarball %s: %s, %v", tarballName, out, err)
	}
	metaPath := path.Join(cwd, modelMetaFileName)
	defer os.Remove(metaPath)
	return loadMeta(metaPath)
}

// DumpDBModelExperimental returns the dumped model tar file name and model meta (JSON serialized).
func DumpDBModelExperimental(db *database.DB, table, cwd string) (string, *Model, error) {
	sqlf, err := sqlfs.Open(db.DB, table)
	if err != nil {
		return "", nil, fmt.Errorf("Can't open sqlfs %s, %v", table, err)
	}
	defer sqlf.Close()
	lengthHexStr := make([]byte, 10)
	n, err := sqlf.Read(lengthHexStr)
	if err != nil || n != 10 {
		return "", nil, fmt.Errorf("read meta length from db error: %v", err)
	}
	metaLength, err := strconv.ParseInt(string(lengthHexStr), 0, 64)
	if err != nil {
		return "", nil, fmt.Errorf("convert length head error: %v", err)
	}
	metaStr := make([]byte, metaLength)
	l, err := sqlf.Read(metaStr)
	if err != nil {
		return "", nil, fmt.Errorf("read meta json from db error: %v", err)
	}
	if int64(l) != metaLength {
		return "", nil, fmt.Errorf("read meta json from db error: invalid meta length read %d", l)
	}

	model := &Model{}
	if model.Meta, err = simplejson.NewJson(metaStr); err != nil {
		return "", nil, fmt.Errorf("model meta json parse error: %v", err)
	}
	model.TrainSelect = model.GetMetaAsString("original_sql")

	fileName := filepath.Join(cwd, "model_dump.tar.gz")
	file, err := os.Create(fileName)
	if err != nil {
		return "", nil, fmt.Errorf("Can't careate model file: %v", err)
	}
	if _, err = io.Copy(file, sqlf); err != nil {
		return "", nil, fmt.Errorf("Can't dump model to local file")
	}
	return fileName, model, nil
}

func loadMeta(metaFileName string) (*Model, error) {
	data, err := ioutil.ReadFile(metaFileName)
	if err != nil {
		return nil, fmt.Errorf("can't read model metadata file")
	}
	model := &Model{}
	if err = decodeMeta(model, data); err != nil {
		return nil, err
	}
	return model, nil
}

func decodeMeta(model *Model, meta []byte) (err error) {
	if model.Meta, err = simplejson.NewJson(meta); err != nil {
		return
	}
	// (NOTE: lhw) we may decode more info later and make them
	// as Model's fields
	// for now, stay compatible with old interface
	model.TrainSelect = model.GetMetaAsString("original_sql")
	return nil
}

// MockInDB mocks a model meta structure which saved in database for testing
func MockInDB(cwd, trainSelect, table string) error {
	m := New(cwd, trainSelect)
	metaStr := fmt.Sprintf(`{"original_sql": "%s"}`, strings.ReplaceAll(strings.ReplaceAll(trainSelect, "\n", " "), `"`, `\"`))
	if e := decodeMeta(m, []byte(metaStr)); e != nil {
		return e
	}
	if e := ioutil.WriteFile(path.Join(cwd, modelMetaFileName), []byte(metaStr), 0644); e != nil {
		return e
	}
	return m.saveDB(database.GetTestingDBSingleton().URL(), table, database.GetSessionFromTestingDB())
}
