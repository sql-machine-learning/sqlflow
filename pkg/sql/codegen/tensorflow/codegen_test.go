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

package tensorflow

import (
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/pkg/sql/codegen"
)

func TestTrainCodegen(t *testing.T) {
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
	TRAIN DNNClassifier
	WITH train.batch_size=4,
		 train.epoch=3,
		 model.hidden_units=[10,20],
		 model.n_classes=3
	COLUMN sepal_length, sepal_width, petal_length, petal_width
	LABEL class
	INTO sqlflow_models.my_xgboost_model;`
	ir := codegen.TrainIR{
		DataSource:       fmt.Sprintf("mysql://%s", cfg.FormatDSN()),
		Select:           "select * from iris.train;",
		ValidationSelect: "select * from iris.test;",
		Estimator:        "DNNClassifier",
		Attributes: map[string]interface{}{
			"train.batch_size":   4,
			"train.epoch":        3,
			"model.hidden_units": []int{10, 20},
			"model.n_classes":    3},
		Features: map[string][]codegen.FeatureColumn{
			"feature_columns": {
				&codegen.NumericColumn{&codegen.FieldMeta{"sepal_length", codegen.Float, "", []int{1}, false, nil}},
				&codegen.NumericColumn{&codegen.FieldMeta{"sepal_width", codegen.Float, "", []int{1}, false, nil}},
				&codegen.NumericColumn{&codegen.FieldMeta{"petal_length", codegen.Float, "", []int{1}, false, nil}},
				&codegen.NumericColumn{&codegen.FieldMeta{"petal_width", codegen.Float, "", []int{1}, false, nil}}}},
		Label: &codegen.NumericColumn{&codegen.FieldMeta{"class", codegen.Int, "", []int{1}, false, nil}}}
	_, err := Train(ir)
	a.NoError(err)

	predIR := codegen.PredictIR{
		DataSource:  fmt.Sprintf("mysql://%s", cfg.FormatDSN()),
		Select:      "select * from iris.test;",
		ResultTable: "iris.predict",
		Attributes:  make(map[string]interface{}),
		TrainIR:     &ir,
	}
	_, err = Pred(predIR)
	a.NoError(err)
}
