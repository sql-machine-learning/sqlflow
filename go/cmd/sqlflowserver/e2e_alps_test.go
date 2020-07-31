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

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	server "sqlflow.org/sqlflow/go/sqlflowserver"
)

func TestEnd2EndMaxComputeALPS(t *testing.T) {
	testDBDriver := os.Getenv("SQLFLOW_TEST_DB")
	modelDir, _ := ioutil.TempDir("/tmp", "sqlflow_ssl_")
	defer os.RemoveAll(modelDir)
	tmpDir, caCrt, caKey, err := generateTempCA()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatalf("failed to generate CA pair %v", err)
	}
	submitter := os.Getenv("SQLFLOW_submitter")
	if submitter != "alps" {
		t.Skip("Skip, this test is for maxcompute + alps")
	}

	if testDBDriver != "maxcompute" {
		t.Skip("Skip maxcompute tests")
	}
	AK := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_AK")
	SK := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_SK")
	endpoint := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_ENDPOINT")
	dbConnStr = fmt.Sprintf("maxcompute://%s:%s@%s", AK, SK, endpoint)

	caseDB = os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
	if caseDB == "" {
		t.Fatalf("Must set env SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT when testing ALPS cases (SQLFLOW_submitter=alps)!!")
	}

	go start(modelDir, caCrt, caKey, unitTestPort, false)
	server.WaitPortReady(fmt.Sprintf("localhost:%d", unitTestPort), 0)

	t.Run("CaseTrainALPS", CaseTrainALPS)
	// TODO(typhoonzero): add this back later
	// t.Run("CaseTrainALPSFeatureMap", CaseTrainALPSFeatureMap)
	// t.Run("CaseTrainALPSRemoteModel", CaseTrainALPSRemoteModel)
}

// CaseTrainALPS is a case for training models using ALPS with out feature_map table
func CaseTrainALPS(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s.sqlflow_test_iris_train
TO TRAIN DNNClassifier
WITH
	model.n_classes = 3,
	model.hidden_units = [10, 20],
	train.batch_size = 10,
	validation.select = "SELECT * FROM %s.sqlflow_test_iris_test"
LABEL class
INTO model_table;`, caseDB, caseDB)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

// CaseTrainALPSRemoteModel is a case for training models using ALPS with remote model
func CaseTrainALPSRemoteModel(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT deep_id, user_space_stat, user_behavior_stat, space_stat, l
FROM %s.sparse_column_test
LIMIT 100
TO TRAIN models.estimator.dnn_classifier.DNNClassifier
WITH 
	model.n_classes = 2, model.hidden_units = [10, 20], train.batch_size = 10, engine.ps_num=0, engine.worker_num=0, engine.type=local,
	gitlab.project = "Alps/sqlflow-models",
	gitlab.source_root = python,
	gitlab.token = "%s"
COLUMN SPARSE(deep_id,15033,COMMA,int),
       SPARSE(user_space_stat,310,COMMA,int),
       SPARSE(user_behavior_stat,511,COMMA,int),
       SPARSE(space_stat,418,COMMA,int),
       EMBEDDING(CATEGORY_ID(deep_id,15033,COMMA),512,mean),
       EMBEDDING(CATEGORY_ID(user_space_stat,310,COMMA),64,mean),
       EMBEDDING(CATEGORY_ID(user_behavior_stat,511,COMMA),64,mean),
       EMBEDDING(CATEGORY_ID(space_stat,418,COMMA),64,mean)
LABEL l
INTO model_table;`, caseDB, os.Getenv("GITLAB_TOKEN"))
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

// CaseTrainALPSFeatureMap is a case for training models using ALPS with feature_map table
func CaseTrainALPSFeatureMap(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT dense, deep, item, test_sparse_with_fm.label
FROM %s.test_sparse_with_fm
LIMIT 32
TO TRAIN alipay.SoftmaxClassifier
WITH train.max_steps = 32, eval.steps=32, train.batch_size=8, engine.ps_num=0, engine.worker_num=0, engine.type = local
COLUMN DENSE(dense, none, comma),
       DENSE(item, 1, comma, int)
LABEL "label" INTO model_table;`, caseDB)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}
