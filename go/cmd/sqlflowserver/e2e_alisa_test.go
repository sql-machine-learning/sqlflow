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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	server "sqlflow.org/sqlflow/go/sqlflowserver"
)

// TestEnd2EndAlisa test cases that run on Alisa, Need to set the
// below environment variables to run them:
// SQLFLOW_submitter=alisa
// SQLFLOW_TEST_DATASOURCE="xxx"
// SQLFLOW_OSS_CHECKPOINT_CONFIG="xxx"
// SQLFLOW_OSS_ALISA_ENDPOINT="xxx"
// SQLFLOW_OSS_AK="xxx"
// SQLFLOW_OSS_SK="xxx"
// SQLFLOW_OSS_ALISA_BUCKET="xxx"
// SQLFLOW_OSS_MODEL_ENDPOINT="xxx"
func TestEnd2EndAlisa(t *testing.T) {
	testDBDriver := os.Getenv("SQLFLOW_TEST_DB")
	if testDBDriver != "alisa" {
		t.Skip("Skipping non alisa tests")
	}
	if os.Getenv("SQLFLOW_submitter") != "alisa" {
		t.Skip("Skip non Alisa tests")
	}
	dbConnStr = os.Getenv("SQLFLOW_TEST_DATASOURCE")
	tmpDir, caCrt, caKey, err := generateTempCA()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatalf("failed to generate CA pair %v", err)
	}
	caseDB = os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
	if caseDB == "" {
		t.Fatalf("Must set env SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
	}
	caseTrainTable = caseDB + ".sqlflow_test_iris_train"
	caseTestTable = caseDB + ".sqlflow_test_iris_test"
	casePredictTable = caseDB + ".sqlflow_test_iris_predict"
	// write model to current MaxCompute project
	caseInto = "sqlflow_test_kmeans_model"

	go start("", caCrt, caKey, unitTestPort, false)
	server.WaitPortReady(fmt.Sprintf("localhost:%d", unitTestPort), 0)
	// TODO(Yancey1989): reuse CaseTrainXGBoostOnPAI if support explain XGBoost model
	t.Run("CaseTrainXGBoostOnAlisa", CaseTrainXGBoostOnAlisa)
	t.Run("CaseTrainPAIKMeans", CaseTrainPAIKMeans)
}

func CaseTrainXGBoostOnAlisa(t *testing.T) {
	a := assert.New(t)
	model := "my_xgb_class_model"
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
	TO TRAIN xgboost.gbtree
	WITH
		objective="multi:softprob",
		train.num_boost_round = 30,
		eta = 0.4,
		num_class = 3
	LABEL class
	INTO %s;`, caseTrainTable, model)
	if _, _, _, err := connectAndRunSQL(trainSQL); err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT * FROM %s
	TO PREDICT %s.class
	USING %s;`, caseTestTable, casePredictTable, model)
	if _, _, _, err := connectAndRunSQL(predSQL); err != nil {
		a.Fail("Run predSQL error: %v", err)
	}

	explainSQL := fmt.Sprintf(`SELECT * FROM %s
	TO EXPLAIN %s
	WITH label_col=class
	USING TreeExplainer
	INTO my_xgb_explain_result;`, caseTrainTable, model)
	if _, _, _, err := connectAndRunSQL(explainSQL); err != nil {
		a.Fail("Run predSQL error: %v", err)
	}
}
