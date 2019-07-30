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
	filler := &xgboostFiller{}
	tmpMap := make(map[string][]string)
	setPair := func(k string, v string) { tmpMap[k] = []string{v} }
	assertEq := func(m map[string]interface{}, key string, refVal interface{}) {
		val, _ := m[key]
		a.EqualValues(refVal, val)
	}

	setPair("run_local", "true")
	setPair("workers", "11")
	setPair("memory", "8192")
	setPair("cpu", "4")
	setPair("objective", "binary:logistic")
	setPair("booster", "gblinear")
	setPair("max_depth", "5")
	setPair("num_class", "2")
	setPair("eta", "0.03")
	setPair("tree_method", "hist")
	setPair("subsample", "0.8")
	setPair("colsample_bytree", "0.5")
	setPair("colsample_bylevel", "0.6")
	setPair("max_bin", "128")
	setPair("verbosity", "3")
	setPair("num_round", "300")
	setPair("auto_train", "true")
	setPair("detail_column", "prediction_detail")
	setPair("prob_column", "prediction_probability")
	setPair("encoding_column", "prediction_leafs")
	setPair("result_column", "prediction_results")
	tmpMap["append_columns"] = []string{"AA", "BB", "CC"}

	e := setXGBoostAttr(&tmpMap, filler)
	a.NoError(e)
	a.Equal(len(tmpMap), 0)

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
	// FIXME: sperlingxx
	//a := assert.New(t)
	//filler := &xgboostFiller{}
	//parser := newParser()
	//sqlHead := `select a, b, c, d, e from table_x
	//			TRAIN XGBoostEstimator`
	//sqlTail := `LABEL e
	//			INTO model_table;`
	//
	//sparseKVSpec := `COLUMN
	//					SPARSE(a, 100, comma)`
	//r, e := parser.Parse(sqlHead + "\n" + sparseKVSpec + "\n" + sqlTail)
	//a.NoError(e)
	//fc, _ := r.columns["feature_columns"]
	//e = parseFeatureColumns(&fc, filler)
	//a.NoError(e)
	//a.EqualValues(100, filler.FeatureSize)
	//a.EqualValues(",", filler.Delimiter)
	//a.EqualValues(true, filler.IsSparse)
	//a.EqualValues([]string{"a"}, filler.FeatureColumns)
	//a.EqualValues("a", filler.X[0].FeatureName)
	//a.EqualValues("string", filler.X[0].Dtype)
	//a.EqualValues("", filler.X[0].Delimiter)
	//a.EqualValues("", filler.X[0].InputShape)
	//a.EqualValues(false, filler.X[0].IsSparse)
	//a.EqualValues("", filler.X[0].FeatureColumnCode)
	//a.EqualValues(false, filler.IsTensorFlowIntegrated)
	//
	//rawColumnsSpec := "a, b, b, c, d, c"
	//r, e = parser.Parse(sqlHead + rawColumnsSpec + sqlTail)
	//a.NoError(e)
	//fc, _ = r.columns["feature_columns"]
	//e = parseFeatureColumns(&fc, filler)
	//a.NoError(e)
	//a.EqualValues(6, filler.FeatureSize)
	//a.EqualValues("", filler.Delimiter)
	//a.EqualValues(false, filler.IsSparse)
	//feaKeys := []string{"a", "b", "b", "c", "d", "c"}
	//a.EqualValues(feaKeys, filler.FeatureColumns)
	//for i, key := range feaKeys {
	//	a.EqualValues(key, filler.X[i].FeatureName)
	//	a.EqualValues("float32", filler.X[i].Dtype)
	//	a.EqualValues("", filler.X[i].Delimiter)
	//	a.EqualValues("[1]", filler.X[i].InputShape)
	//	a.EqualValues(false, filler.X[i].IsSparse)
	//	a.EqualValues("", filler.X[i].FeatureColumnCode)
	//}
	//a.EqualValues(false, filler.IsTensorFlowIntegrated)
}
