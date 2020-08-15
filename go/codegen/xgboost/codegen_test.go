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

package xgboost

import (
	"os"
	"reflect"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

func TestParseAttribute(t *testing.T) {
	a := assert.New(t)
	params := parseAttribute(map[string]interface{}{"a": "b", "c": "d", "train.e": "f"})
	a.True(reflect.DeepEqual(map[string]interface{}{"a": "b", "c": "d"}, params[""]))
	a.True(reflect.DeepEqual(map[string]interface{}{"e": "f"}, params["train."]))
}

func TestAttributes(t *testing.T) {
	a := assert.New(t)
	a.Equal(10, len(attributeDictionary))
	a.Equal(33, len(fullAttrValidator))
}

func mockSession() *pb.Session {
	return &pb.Session{DbConnStr: database.GetTestingMySQLURL()}
}

func TestTrainAndPredict(t *testing.T) {
	driver := os.Getenv("SQLFLOW_TEST_DB")
	a := assert.New(t)
	tir := ir.MockTrainStmt(true)
	a.NoError(InitializeAttributes(tir))
	_, err := Train(tir, mockSession())
	a.NoError(err)

	pir := ir.MockPredStmt(tir)
	sess := &pb.Session{
		Token:        "",
		DbConnStr:    "",
		ExitOnSubmit: false,
		UserId:       "",
	}
	if driver == "hive" {
		sess.DbConnStr = database.GetTestingHiveURL()
	}
	code, err := Pred(pir, sess)
	a.NoError(err)

	if driver == "hive" {
		r, _ := regexp.Compile(`hive_location=(.*)`)
		a.Contains(r.FindStringSubmatch(code)[1], "/sqlflow")
		r, _ = regexp.Compile(`hdfs_namenode_addr=(.*)&`)
		a.Equal(r.FindStringSubmatch(code)[1], "")
	} else {
		r, _ := regexp.Compile(`hive_location=(.*)`)
		a.Equal(0, len(r.FindStringSubmatch(code)))
	}

}

func TestResolveModelParams(t *testing.T) {
	a := assert.New(t)
	shortName := []string{"XGBOOST.XGBCLASSIFIER", "XGBOOST.XGBREGRESSOR", "XGBRANKER"}
	objectiveName := []string{"binary:logistic", "reg:squarederror", "rank:pairwise"}
	for i := range shortName {
		tir := ir.MockTrainStmt(true)
		tir.Estimator = shortName[i]
		delete(tir.Attributes, "objective")
		err := resolveModelParams(tir)
		a.NoError(err)
		a.Equal(objectiveName[i], tir.Attributes["objective"])
	}
}

func TestTrainWithModelRepoImage(t *testing.T) {
	a := assert.New(t)
	tir := ir.MockTrainStmt(true)
	a.NoError(InitializeAttributes(tir))
	tir.ModelImage = "myRepo/MyXGBClassifier:v1.0"
	code, err := Train(tir, mockSession())
	a.NoError(err)
	r, _ := regexp.Compile(`model_repo_image="(.*)"`)
	a.Equal(r.FindStringSubmatch(code)[1], tir.ModelImage)

	// dist train
	code, err = DistTrain(tir, mockSession(), 2, "", "")
	a.NoError(err)
	r, _ = regexp.Compile(`model_repo_image="(.*)"`)
	a.Equal(r.FindStringSubmatch(code)[1], tir.ModelImage)

}
