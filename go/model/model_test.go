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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/argoproj/pkg/file"
	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/sqlfs"
)

const modelMeta = `
{
  "select": "SELECT * FROM iris.train where class!=2",
  "validate_select": "SELECT * FROM iris.test where class!=2",
  "estimator": "tf.estimator.BoostedTreesClassifier",
  "attributes": {
    "n_batches_per_layer": 1,
    "n_classes": 2,
    "n_trees": 50,
    "center_bias": true
  },
  "feature_columns": "",
  "field_descs": {
    "sepal_length": {
      "feature_name": "sepal_length",
      "dtype": "float32",
      "delimiter": "",
      "shape": [ 1 ],
      "is_sparse": false,
      "name": "sepal_length"
    }
  },
  "label": {
    "feature_name": "class",
    "dtype": "int64",
    "delimiter": "",
    "shape": [],
    "is_sparse": false
  },
  "evaluation": null
}`

func mockModelDir(a *assert.Assertions) (string, string) {
	ws, err := ioutil.TempDir("/tmp", "model_ws")
	a.NoError(err)
	err = ioutil.WriteFile(path.Join(ws, "model.txt"), []byte("model data"), 0644)
	a.NoError(err)
	err = ioutil.WriteFile(path.Join(ws, modelMetaFileName), []byte(modelMeta), 0644)
	a.NoError(err)

	dst, err := ioutil.TempDir("/tmp", "dst")
	a.NoError(err)

	return ws, dst
}

func TestModelFileStore(t *testing.T) {
	a := assert.New(t)
	ws, dst := mockModelDir(a)
	defer os.RemoveAll(ws)
	defer os.RemoveAll(dst)
	model := &Model{workDir: ws}
	session := database.GetSessionFromTestingDB()
	modelURI := fmt.Sprintf("file://%s/model", dst)

	err := model.Save(modelURI, session)
	a.NoError(err)
	a.True(file.Exists(path.Join(dst, "model.tar.gz")))

	model, err = Load(modelURI, dst, nil)
	a.NoError(err)
	a.True(file.Exists(path.Join(dst, "model.txt")))
	a.True(file.Exists(path.Join(dst, modelMetaFileName)))
	meta, err := ioutil.ReadFile(path.Join(dst, modelMetaFileName))
	a.NoError(err)
	a.Equal(modelMeta, string(meta))
	a.Equal("tf.estimator.BoostedTreesClassifier", model.GetMetaAsString("estimator"))
}

func TestModelDBStore(t *testing.T) {
	a := assert.New(t)
	ws, dst := mockModelDir(a)
	defer os.RemoveAll(ws)
	defer os.RemoveAll(dst)
	model := &Model{workDir: ws}
	session := database.GetSessionFromTestingDB()

	table := "iris.my_boost_tree_model"
	err := model.Save(table, session)
	a.NoError(err)

	db, err := database.OpenAndConnectDB(session.DbConnStr)
	a.NoError(err)
	defer db.Close()
	model, err = Load(table, dst, db)
	a.NoError(err)
	a.True(file.Exists(path.Join(dst, "model.txt")))
	a.True(file.Exists(path.Join(dst, modelMetaFileName)))
	meta, err := ioutil.ReadFile(path.Join(dst, modelMetaFileName))
	a.NoError(err)
	a.Equal(modelMeta, string(meta))
	a.Equal("tf.estimator.BoostedTreesClassifier", model.GetMetaAsString("estimator"))

	// only load meta
	model, err = Load(table, "", db)
	a.NoError(err)
	a.Equal("tf.estimator.BoostedTreesClassifier", model.GetMetaAsString("estimator"))
	a.Equal("SELECT * FROM iris.train where class!=2", model.GetMetaAsString("select"))
}

func TestDumpDBModelExperimental(t *testing.T) {
	a := assert.New(t)
	metaLen := len(modelMeta)
	metaLenStr := fmt.Sprintf("0x%08x", metaLen)
	a.Equal(684, metaLen)
	a.Equal("0x000002ac", metaLenStr)

	session := database.GetSessionFromTestingDB()
	db, err := database.OpenAndConnectDB(session.DbConnStr)
	table := "iris.test_dump_model_experimental"
	a.NoError(err)
	sqlf, e := sqlfs.Create(db, table, session)
	a.NoError(e)

	sqlf.Write([]byte(metaLenStr))
	sqlf.Write([]byte(modelMeta))
	fakeData := make([]byte, 1024)
	for i := 0; i < 1024; i++ {
		fakeData[i] = 0
	}
	sqlf.Write(fakeData) // fake data
	sqlf.Close()

	ws, err := ioutil.TempDir("/tmp", "model_ws")
	a.NoError(err)
	defer os.RemoveAll(ws)
	fn, model, err := DumpDBModelExperimental(db, table, ws)
	if err != nil {
		a.FailNow("error: %v", err)
	}

	a.Equal(ws+"/model_dump.tar.gz", fn)
	a.Equal("tf.estimator.BoostedTreesClassifier", model.GetMetaAsString("estimator"))
}
