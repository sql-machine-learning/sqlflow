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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	pb "sqlflow.org/sqlflow/pkg/server/proto"
)

const testXGBoostTrainSelectIris = ` 
SELECT *
FROM iris.train
TRAIN xgboost.gbtree
WITH
    objective="multi:softprob",
    train.num_boost_round = 30,
    eta = 3.1,
    num_class = 3
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class 
INTO sqlflow_models.my_xgboost_model;
`
const testAnalyzeTreeModelSelectIris = `
SELECT * FROM iris.train
ANALYZE sqlflow_models.my_xgboost_model
USING TreeExplainer;
`

const testXGBoostPredictIris = ` 
SELECT *
FROM iris.test
PREDICT iris.predict.class
USING sqlflow_models.my_xgboost_model;
`

func TestXGBFiller(t *testing.T) {
	a := assert.New(t)
	parser := newParser()
	r, e := parser.Parse(testXGBoostTrainSelectIris)
	a.NoError(e)
	filler, e := newXGBFiller(r, nil, testDB, nil)
	a.NoError(e)
	a.True(filler.IsTrain)
	a.Equal(filler.NumBoostRound, 30)
	expectedParams := map[string]interface{}{
		"eta":       3.1,
		"num_class": 3,
		"objective": "multi:softprob",
	}
	paramsJSON, err := json.Marshal(expectedParams)
	a.NoError(err)
	a.Equal(filler.ParamsCfgJSON, string(paramsJSON))
}

func TestXGBFillerPredict(t *testing.T) {
	a := assert.New(t)
	parser := newParser()
	r, e := parser.Parse(testXGBoostPredictIris)
	a.NoError(e)

	sess := &pb.Session{
		Token:            "",
		DbConnStr:        testDB.String(),
		ExitOnSubmit:     false,
		UserId:           "",
		HiveLocation:     "/sqlflowtmp",
		HdfsNamenodeAddr: "192.168.1.1:8020",
		HdfsUser:         "hdfs_user",
		HdfsPass:         "hdfs_pass",
	}
	filler, e := newXGBFiller(r, nil, testDB, sess)
	a.NoError(e)
	a.False(filler.IsTrain)
	a.Equal(filler.TableName, "iris.predict")
	a.Equal(filler.Save, "sqlflow_models.my_xgboost_model")
	a.Equal(filler.PredictionDatasetSQL, `SELECT *
FROM iris.test`)
	a.Equal("/sqlflowtmp", filler.HiveLocation)
	a.Equal("192.168.1.1:8020", filler.HDFSNameNodeAddr)
	a.Equal("hdfs_user", filler.HDFSUser)
	a.Equal("hdfs_pass", filler.HDFSPass)
}
