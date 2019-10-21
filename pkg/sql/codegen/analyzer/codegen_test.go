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
// limitations under the License.o

package analyzer

import (
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/pkg/sql/codegen"
)

func TestGenAnalysis(t *testing.T) {
	a := assert.New(t)
	tir := mockTrainIR()
	attrs := make(map[string]interface{})
	attrs["shap_summary.plot_type"] = "dot"
	attrs["others.type"] = "dot"
	air := &codegen.AnalyzeIR{
		DataSource: tir.DataSource,
		Select:     "SELECT * FROM iris.train",
		Explainer:  "TreeExplainer",
		Attributes: attrs,
		TrainIR:    tir,
	}
	_, err := GenAnalysis(air, "")
	a.NoError(err)
}

func mockTrainIR() *codegen.TrainIR {
	cfg := &mysql.Config{
		User:                 "root",
		Passwd:               "root",
		Net:                  "tcp",
		Addr:                 "127.0.0.1:3306",
		AllowNativePasswords: true,
	}
	l := codegen.NumericColumn{FieldMeta: &codegen.FieldMeta{Name: "class"}}
	return &codegen.TrainIR{
		DataSource:       fmt.Sprintf("mysql://%s", cfg.FormatDSN()),
		Select:           "select * from iris.train;",
		ValidationSelect: "select * from iris.test;",
		Estimator:        "xgboost.gbtree",
		Label:            &l,
		Attributes: map[string]interface{}{
			"train.num_boost_round": 10,
			"objective":             "multi:softprob",
			"eta":                   float32(0.1),
			"num_class":             3},
	}
}
