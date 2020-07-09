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

	server "sqlflow.org/sqlflow/go/sqlflowserver"
)

func TestEnd2EndMaxCompute(t *testing.T) {
	testDBDriver := os.Getenv("SQLFLOW_TEST_DB")
	modelDir, _ := ioutil.TempDir("/tmp", "sqlflow_ssl_")
	defer os.RemoveAll(modelDir)
	tmpDir, caCrt, caKey, err := generateTempCA()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatalf("failed to generate CA pair %v", err)
	}
	submitter := os.Getenv("SQLFLOW_submitter")
	if submitter == "alps" || submitter == "elasticdl" {
		t.Skip("Skip this test case, it's for maxcompute + submitters other than alps and elasticdl.")
	}

	if testDBDriver != "maxcompute" {
		t.Skip("Skip maxcompute tests")
	}
	AK := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_AK")
	SK := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_SK")
	endpoint := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_ENDPOINT")
	dbConnStr = fmt.Sprintf("maxcompute://%s:%s@%s", AK, SK, endpoint)
	go start(modelDir, caCrt, caKey, unitTestPort, false)
	server.WaitPortReady(fmt.Sprintf("localhost:%d", unitTestPort), 0)

	caseDB = os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
	caseTrainTable = "sqlflow_test_iris_train"
	caseTestTable = "sqlflow_test_iris_test"
	casePredictTable = "sqlflow_test_iris_predict"
	err = prepareTestData(dbConnStr)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}

	t.Run("caseTrainSQL", caseTrainSQL)

	// FIXME(typhoonzero): add this back later
	// caseTensorFlowIncrementalTrain(t, true)

	caseXGBoostFeatureColumn(t, true)
}
