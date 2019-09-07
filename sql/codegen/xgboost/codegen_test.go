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

package xgboost

import (
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/sql-machine-learning/sqlflow/sql"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTrain(t *testing.T) {
	a := assert.New(t)

	cfg := &mysql.Config{
		User:                 "root",
		Passwd:               "root",
		Net:                  "tcp",
		Addr:                 "127.0.0.1:3306",
		AllowNativePasswords: true,
	}
	_ = `SELECT *
		FROM iris.train
	TRAIN xgboost
	WITH
		train.num_boost_round = 30,
		model.objective = "multi:softprob"
		model.eta = 3.1,
		model.num_class = 3
	COLUMN sepal_length, sepal_width, petal_length, petal_width
	LABEL class
	INTO sqlflow_models.my_xgboost_model;`
	ir := sql.TrainIR{
		DataSource:       fmt.Sprintf("mysql://%s", cfg.FormatDSN()),
		Select:           "select * from iris.train;",
		ValidationSelect: "select * from iris.test;",
		Estimator:        "xgb.multi.softprob",
		Attribute:        map[string]interface{}{"train.num_boost_round": 30, "model.objective": "multi:softprob", "model.eta": 3.1, "model.num_class": 3},
		Feature: map[string]map[string]sql.FieldMeta{
			"feature_columns": {
				"sepal_length": {sql.Float, "", []int{1}, false},
				"sepal_width":  {sql.Float, "", []int{1}, false},
				"petal_length": {sql.Float, "", []int{1}, false},
				"petal_width":  {sql.Float, "", []int{1}, false}}},
		Label: map[string]sql.FieldMeta{"class": {sql.Int, "", []int{1}, false}}}
	_, err := Train(ir)
	a.NoError(err)
}
