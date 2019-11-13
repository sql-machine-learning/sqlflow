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

package main

import (
	"fmt"

	"github.com/go-sql-driver/mysql"
	"sqlflow.org/sqlflow/pkg/sql/codegen"
)

func main() {
	TestTrain()
}

// TestTrain test XGBoost model train
func TestTrain() {
	tir := mockTrainIR()
	p, _ := HPOXGBoostModel(tir)

	fmt.Println(p)
}

// mockTrainIR generate intermediate data for train
func mockTrainIR() *codegen.TrainIR {
	cfg := &mysql.Config{
		User:                 "root",
		Passwd:               "root",
		Net:                  "tcp",
		Addr:                 "127.0.0.1:3306",
		AllowNativePasswords: true,
	}
	_ = `SELECT *
		FROM iris.train
	TRAIN xgboost.gbtree
	WITH
		objective = "multi:softprob"
		eta = 3.1,
		num_class = 3,
		train.num_boost_round = 30
	COLUMN sepal_length, sepal_width, petal_length, petal_width
	LABEL class
	INTO sqlflow_models.my_xgboost_model;`
	return &codegen.TrainIR{
		DataSource:       fmt.Sprintf("mysql://%s", cfg.FormatDSN()),
		Select:           "select * from iris.train;",
		ValidationSelect: "select * from iris.test;",
		Estimator:        "xgboost.gbtree",
		Attributes: map[string]interface{}{
			"objective": "binary:logistic",
			"algorithm": "random"},
		Features: map[string][]codegen.FeatureColumn{
			"feature_columns": {
				&codegen.NumericColumn{&codegen.FieldMeta{"sepal_length", codegen.Float, "", []int{1}, false, nil, 0}},
				&codegen.NumericColumn{&codegen.FieldMeta{"sepal_width", codegen.Float, "", []int{1}, false, nil, 0}},
				&codegen.NumericColumn{&codegen.FieldMeta{"petal_length", codegen.Float, "", []int{1}, false, nil, 0}},
				&codegen.NumericColumn{&codegen.FieldMeta{"petal_width", codegen.Float, "", []int{1}, false, nil, 0}}}},
		Label: &codegen.NumericColumn{&codegen.FieldMeta{"class", codegen.Int, "", []int{1}, false, nil, 0}}}
}
