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

	"sqlflow.org/sqlflow/go/database"
	server "sqlflow.org/sqlflow/go/sqlflowserver"
)

func TestEnd2EndHive(t *testing.T) {
	testDBDriver := os.Getenv("SQLFLOW_TEST_DB")
	modelDir := ""
	tmpDir, caCrt, caKey, err := generateTempCA()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatalf("failed to generate CA pair %v", err)
	}

	if testDBDriver != "hive" {
		t.Skip("Skipping hive tests")
	}
	dbConnStr = database.GetTestingHiveURL()
	go start(modelDir, caCrt, caKey, unitTestPort, false)
	server.WaitPortReady(fmt.Sprintf("localhost:%d", unitTestPort), 0)
	err = prepareTestData(dbConnStr)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}
	t.Run("caseShowDatabases", caseShowDatabases)
	t.Run("caseSelect", caseSelect)
	t.Run("caseTrainSQL", caseTrainSQL)
	t.Run("caseCoverageCommon", caseCoverageCommon)
	t.Run("caseCoverageCustomModel", caseCoverageCustomModel)
	// cases that need to check results:
	t.Run("caseTrainRegression", caseTrainRegression)
	t.Run("caseTrainXGBoostRegressionConvergence", caseTrainXGBoostRegressionConvergence)
	t.Run("casePredictXGBoostRegression", casePredictXGBoostRegression)
	// t.Run("CaseTrainFeatureDerivation", caseTrainFeatureDerivation)
	t.Run("caseShowTrain", caseShowTrain)

	caseTensorFlowIncrementalTrain(t, false)

	caseXGBoostFeatureColumn(t, false)

	t.Run("CaseXGBoostSparseKeyValueColumn", caseXGBoostSparseKeyValueColumn)

	// Cases for optimize
	t.Run("CaseTestOptimizeClauseWithoutGroupBy", caseTestOptimizeClauseWithoutGroupBy)
	t.Run("CaseTestOptimizeClauseWithGroupBy", caseTestOptimizeClauseWithGroupBy)
	t.Run("CaseTestOptimizeClauseWithBinaryVarType", caseTestOptimizeClauseWithBinaryVarType)
	t.Run("CaseTestOptimizeClauseWithoutConstraint", caseTestOptimizeClauseWithoutConstraint)
}
