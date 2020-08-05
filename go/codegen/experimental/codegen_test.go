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

package experimental

import (
	"os"
	"testing"

	"sqlflow.org/sqlflow/go/database"
	pb "sqlflow.org/sqlflow/go/proto"
)

func TestExperimentalXGBCodegen(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST_DB") != "mysql" {
		t.Skipf("skip TestExperimentalXGBCodegen of DB type %s", os.Getenv("SQLFLOW_TEST_DB"))
	}
	// test without COLUMN clause
	sql := "SELECT * FROM iris.train TO TRAIN xgboost.gbtree WITH objective=\"binary:logistic\",num_class=3 LABEL class INTO sqlflow_models.xgb_classification;"
	s := &pb.Session{DbConnStr: database.GetTestingMySQLURL()}
	_, err := GenerateCodeCouler(sql, s)
	if err != nil {
		t.Errorf("error %s", err)
	}

	// test with COLUMN clause
	sql = "SELECT * FROM iris.train TO TRAIN xgboost.gbtree WITH objective=\"binary:logistic\",num_class=3 COLUMN petal_length LABEL class INTO sqlflow_models.xgb_classification;"
	_, err = GenerateCodeCouler(sql, s)
	if err != nil {
		t.Errorf("error %s", err)
	}
}
