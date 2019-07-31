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
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestPartials(t *testing.T) {
	a := assert.New(t)
	tmpMap := make(map[string][]string)
	filler := &xgboostFiller{}

	// test strPartial
	part := strPartial("obj", func(r *xgboostFiller) *string { return &(r.Objective) })
	tmpMap["obj"] = []string{"binary:logistic"}
	e := part(&tmpMap, filler)
	a.NoError(e)
	a.Equal(filler.Objective, "binary:logistic")
	_, ok := tmpMap["obj"]
	a.Equal(ok, false)
	// Error: duplicate attr setting
	tmpMap["obj"] = []string{"binary:logistic"}
	e = part(&tmpMap, filler)
	a.Error(e)
	// Error: len(val) > 1
	tmpMap["obj"] = []string{"binary:logistic", "reg:linear"}
	e = part(&tmpMap, filler)
	a.Error(e)
	//  change objective to "reg:linear"
	tmpMap["obj"] = []string{"reg:linear"}
	filler.Objective = ""
	e = part(&tmpMap, filler)
	a.NoError(e)
	a.Equal(filler.Objective, "reg:linear")

	// test uIntPartial
	part = uIntPartial("workers", func(r *xgboostFiller) *uint { return &(r.WorkerNum) })
	tmpMap["workers"] = []string{"10"}
	e = part(&tmpMap, filler)
	a.NoError(e)
	a.EqualValues(filler.WorkerNum, 10)
	_, ok = tmpMap["workers"]
	a.Equal(ok, false)

	// test fp32Partial
	part = fp32Partial("eta", func(r *xgboostFiller) *float32 { return &(r.Eta) })
	tmpMap["eta"] = []string{"-0.33"}
	e = part(&tmpMap, filler)
	a.NoError(e)
	a.EqualValues(filler.Eta, float32(-0.33))
	_, ok = tmpMap["eta"]
	a.Equal(ok, false)

	// test boolPartial
	part = boolPartial("run_local", func(r *xgboostFiller) *bool { return &(r.runLocal) })
	tmpMap["run_local"] = []string{"false"}
	e = part(&tmpMap, filler)
	a.NoError(e)
	a.Equal(filler.runLocal, false)
	_, ok = tmpMap["run_local"]
	a.Equal(ok, false)
	tmpMap["run_local"] = []string{"true"}
	e = part(&tmpMap, filler)
	a.NoError(e)
	a.Equal(filler.runLocal, true)

	// test sListPartial
	part = sListPartial("app_col", func(r *xgboostFiller) *[]string { return &(r.AppendColumns) })
	tmpMap["app_col"] = []string{"AA", "BB", "CC"}
	e = part(&tmpMap, filler)
	a.NoError(e)
	a.EqualValues(filler.AppendColumns, []string{"AA", "BB", "CC"})
	_, ok = tmpMap["app_col"]
	a.Equal(ok, false)
}

func TestXGBoostAttr(t *testing.T) {
	a := assert.New(t)
	assertEq := func(m map[string]interface{}, key string, refVal interface{}) {
		val, _ := m[key]
		a.EqualValues(refVal, val)
	}
	parser := newParser()

	filler := &xgboostFiller{}
	testClause := `
SELECT a, b, c, d, e FROM table_xx
TRAIN XGBoostEstimator
WITH
	run_local = true,
	workers = 11,
	memory = 8192,
	cpu = 4,
	objective = "binary:logistic",
	booster = gblinear,
	num_class = 2,
	max_depth = 5,
	eta = 0.03,
	tree_method = hist,
	subsample = 0.8,
	colsample_bytree = 0.5,
	colsample_bylevel = 0.6,
	max_bin = 128,
	verbosity = 3,
	num_round = 300,
	auto_train = true,
	detail_column = "prediction_detail",
	prob_column = "prediction_probability",
	encoding_column = "prediction_leafs",
	result_column = "prediction_results",
	append_columns = ["AA", "BB", "CC"]
COLUMN a, b, c, d
LABEL e INTO model_table;
`
	r, e := parser.Parse(testClause)
	a.NoError(e)
	e = xgParseAttr(r, filler)
	a.NoError(e)

	data, e := json.Marshal(filler.xgboostFields)
	a.NoError(e)
	mapData := make(map[string]interface{})
	e = json.Unmarshal(data, &mapData)
	a.NoError(e)
	params, _ := mapData["params"]
	paramMap, _ := params.(map[string]interface{})
	assertEq(paramMap, "objective", "binary:logistic")
	assertEq(paramMap, "booster", "gblinear")
	assertEq(paramMap, "num_class", 2)
	assertEq(paramMap, "max_depth", 5)
	assertEq(paramMap, "eta", 0.03)
	assertEq(paramMap, "tree_method", "hist")
	assertEq(paramMap, "subsample", 0.8)
	assertEq(paramMap, "colsample_bytree", 0.5)
	assertEq(paramMap, "colsample_bylevel", 0.6)
	assertEq(paramMap, "max_bin", 128)
	assertEq(paramMap, "verbosity", 3)
	assertEq(mapData, "num_boost_round", 300)
	assertEq(mapData, "auto_train", true)

	data, e = json.Marshal(filler.xgColumnFields)
	a.NoError(e)
	mapData = make(map[string]interface{})
	e = json.Unmarshal(data, &mapData)
	a.NoError(e)
	ret, _ := mapData["result_columns"]
	retMap, _ := ret.(map[string]interface{})
	assertEq(retMap, "result_column", "prediction_results")
	assertEq(retMap, "probability_column", "prediction_probability")
	assertEq(retMap, "detail_column", "prediction_detail")
	assertEq(retMap, "leaf_column", "prediction_leafs")
	assertEq(mapData, "append_columns", []interface{}{"AA", "BB", "CC"})

	data, e = json.Marshal(filler.xgRuntimeResourceFields)
	mapData = make(map[string]interface{})
	e = json.Unmarshal(data, &mapData)
	a.NoError(e)
	assertEq(mapData, "worker_num", 11)
	assertEq(mapData, "memory_size", 8192)
	assertEq(mapData, "cpu_size", 4)
}

func TestColumnClause(t *testing.T) {
	a := assert.New(t)
	parser := newParser()
	sqlHead := `
SELECT a, b, c, d, e FROM table_xx
TRAIN XGBoostEstimator
WITH attr_x = XXX
`
	sqlTail := `
LABEL e INTO model_table;
`
	// test sparseKV schema
	filler := &xgboostFiller{}
	sparseKVSpec := ` COLUMN SPARSE(a, 100, comma) `
	r, e := parser.Parse(sqlHead + sparseKVSpec + sqlTail)
	a.NoError(e)
	e = xgParseColumns(r, filler)
	a.NoError(e)
	a.EqualValues(100, filler.FeatureSize)
	a.EqualValues(",", filler.Delimiter)
	a.EqualValues(true, filler.IsSparse)
	a.EqualValues([]string{"a"}, filler.FeatureColumns)
	a.EqualValues("a", filler.X[0].FeatureName)
	a.EqualValues("string", filler.X[0].Dtype)
	a.EqualValues("", filler.X[0].Delimiter)
	a.EqualValues("", filler.X[0].InputShape)
	a.EqualValues(false, filler.X[0].IsSparse)
	a.EqualValues("", filler.X[0].FeatureColumnCode)
	a.EqualValues(false, filler.IsTensorFlowIntegrated)
	a.EqualValues(&xgFeatureMeta{FeatureName: "e"}, filler.LabelField)
	a.EqualValues("e", filler.Label)

	// test raw columns
	filler = &xgboostFiller{}
	rawColumnsSpec := " COLUMN a, b, b, c, d, c "
	r, _ = parser.Parse(sqlHead + rawColumnsSpec + sqlTail)
	e = xgParseColumns(r, filler)
	a.NoError(e)
	a.EqualValues(6, int(filler.FeatureSize))
	a.EqualValues("", filler.Delimiter)
	a.False(filler.IsSparse)
	a.False(filler.IsTensorFlowIntegrated)
	feaKeys := []string{"a", "b", "b", "c", "d", "c"}
	a.EqualValues(feaKeys, filler.FeatureColumns)
	for i, key := range feaKeys {
		a.EqualValues(key, filler.X[i].FeatureName)
		a.EqualValues("float32", filler.X[i].Dtype)
		a.EqualValues("", filler.X[i].Delimiter)
		a.EqualValues("[1]", filler.X[i].InputShape)
		a.EqualValues(false, filler.X[i].IsSparse)
		a.EqualValues("", filler.X[i].FeatureColumnCode)
	}

	// test tf.feature_columns
	filler = &xgboostFiller{}
	fcSpec := " COLUMN a, b, c, EMBEDDING(CATEGORY_ID(d, 2000), 8, mean) FOR feature_columns "
	r, _ = parser.Parse(sqlHead + fcSpec + sqlTail)
	e = xgParseColumns(r, filler)
	a.NoError(e)
	a.EqualValues(0, int(filler.FeatureSize))
	a.EqualValues("", filler.Delimiter)
	a.False(filler.IsSparse)
	a.True(filler.IsTensorFlowIntegrated)

	// test group & weight
	filler = &xgboostFiller{}
	groupWeightSpec := " COLUMN gg FOR group COLUMN ww FOR weight "
	r, _ = parser.Parse(sqlHead + fcSpec + groupWeightSpec + sqlTail)
	e = xgParseColumns(r, filler)
	a.NoError(e)
	a.EqualValues(&xgFeatureMeta{FeatureName: "gg"}, filler.GroupField)
	a.EqualValues("gg", filler.Group)
	a.EqualValues(&xgFeatureMeta{FeatureName: "ww"}, filler.WeightField)
	a.EqualValues("ww", filler.Weight)

	// test xgMixSchemaError
	filler = &xgboostFiller{}
	wrongColSpec := " COLUMN SPARSE(a, 2000, comma), b, c, d "
	r, _ = parser.Parse(sqlHead + wrongColSpec + sqlTail)
	e = xgParseColumns(r, filler)
	a.Error(e)
	a.EqualValues(e, xgParseColumnError("feature_columns", xgMixSchemaError()))

	// test `DENSE` keyword
	filler = &xgboostFiller{}
	wrongColSpec = " COLUMN DENSE(b, 5, comma) "
	r, _ = parser.Parse(sqlHead + wrongColSpec + sqlTail)
	e = xgParseColumns(r, filler)
	a.Error(e)
	a.EqualValues(e, xgParseColumnError("feature_columns", xgUnknownFCError("DENSE")))

	// test xgMultiSparseError
	filler = &xgboostFiller{}
	wrongColSpec = " COLUMN SPARSE(a, 2000, comma), SPARSE(b, 100, comma) "
	r, _ = parser.Parse(sqlHead + wrongColSpec + sqlTail)
	e = xgParseColumns(r, filler)
	a.Error(e)
	a.EqualValues(e, xgParseColumnError("feature_columns", xgMultiSparseError([]string{"a", "b"})))

	// test xgUnsupportedColTagError
	filler = &xgboostFiller{}
	unsupportedSpec := " COLUMN gg FOR group COLUMN ww FOR xxxxx "
	r, _ = parser.Parse(sqlHead + fcSpec + unsupportedSpec + sqlTail)
	e = xgParseColumns(r, filler)
	a.Error(e)
	a.EqualValues(e, xgParseColumnError("xxxxx", xgUnsupportedColTagError()))
}

func TestXGBoostFiller(t *testing.T) {
	a := assert.New(t)
	parser := newParser()
	testClause := `
SELECT * FROM iris.train
TRAIN XGBoostRegressor
WITH
	run_local = true,
	max_depth = 5,
	eta = 0.03,
	tree_method = "hist",
	num_round = 300,
	append_columns = ["A", B, "C"]
COLUMN sepal_length, sepal_width, petal_length, petal_width
COLUMN gg FOR group 
COLUMN ww FOR weight
LABEL e INTO model_table;
`
	pr, e := parser.Parse(testClause)
	a.NoError(e)
	fts, e := verify(pr, testDB)
	a.NoError(e)

	filler, e := newXGBoostFiller(pr, fts, testDB)
	a.NoError(e)
	a.True(filler.isTrain)
	a.EqualValues("SELECT * FROM iris.train;", strings.Replace(filler.standardSelect, "\n", " ", -1))
	a.EqualValues("model_table", filler.modelPath)
	a.True(filler.runLocal)

	a.EqualValues("reg:squarederror", filler.Objective)
	a.EqualValues(0.03, filler.Eta)
	a.EqualValues(5, filler.MaxDepth)
	a.EqualValues("hist", filler.TreeMethod)
	a.EqualValues(300, filler.NumRound)
	a.EqualValues([]string{"A", "B", "C"}, filler.AppendColumns)

	a.EqualValues("e", filler.Label)
	a.EqualValues("e", filler.LabelField.FeatureName)
	a.EqualValues("gg", filler.Group)
	a.EqualValues("gg", filler.GroupField.FeatureName)
	a.EqualValues("ww", filler.Weight)
	a.EqualValues("ww", filler.WeightField.FeatureName)

	a.False(filler.IsTensorFlowIntegrated)
	a.False(filler.IsSparse)
	a.EqualValues("", filler.Delimiter)
	a.EqualValues(4, filler.FeatureSize)
	a.EqualValues([]string{"sepal_length", "sepal_width", "petal_length", "petal_width"}, filler.FeatureColumns)
	a.EqualValues(&xgFeatureMeta{FeatureName: "sepal_length", Dtype: "float32", InputShape: "[1]"}, filler.X[0])
	a.EqualValues(&xgFeatureMeta{FeatureName: "sepal_width", Dtype: "float32", InputShape: "[1]"}, filler.X[1])
	a.EqualValues(&xgFeatureMeta{FeatureName: "petal_length", Dtype: "float32", InputShape: "[1]"}, filler.X[2])
	a.EqualValues(&xgFeatureMeta{FeatureName: "petal_width", Dtype: "float32", InputShape: "[1]"}, filler.X[3])
}
