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
)

const testXGBoostTrainSelectIris = ` 
SELECT *
FROM iris.train
TRAIN xgb.multi.softprob
WITH
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

func TestXGBFiller(t *testing.T) {
	a := assert.New(t)
	parser := newParser()
	r, e := parser.Parse(testXGBoostTrainSelectIris)
	a.NoError(e)
	filler, e := newXGBFiller(r, nil, testDB)
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
