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

package ir

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/sql/testdata"
)

func TestCSVRegex(t *testing.T) {
	csvStings := []string{
		"1,2,3,4",
		"1.3,-3.2,132,32",
		"33,-33",
		"33,-33,",
		" 33 , -70 , 80 , ",
		" 33 , -70 , 80 ,",
		" 33 , -70 , 80, ",
		" 33 , -70 , 80,",
	}
	for _, s := range csvStings {
		if inferStringDataFormat(s, "", "") != csv {
			t.Errorf("%s is not matched", s)
		}
	}
	nonCSVStings := []string{
		"100",
		"-100",
		"023,",
		",123",
		"1.23",
	}
	for _, s := range nonCSVStings {
		if inferStringDataFormat(s, "", "") == csv {
			t.Errorf("%s should not be matched", s)
		}
	}
}

func TestKeyValueRegex(t *testing.T) {
	libsvmKeyValueStrings := []string{
		"1:3 2:4\t 3:5  4:9",
		"0:1.3 10:-3.2 20:132 7:32",
		"3:33",
	}
	for _, s := range libsvmKeyValueStrings {
		if inferStringDataFormat(s, "", "") != kv {
			t.Errorf("%s is not matched", s)
		}
	}
	generalKeyValueStrings := []string{
		"1:3,3:4.0,8:3.0",
		"k:1.0,b:3.3,s:4.32",
		"unknown", // will be parsed to {"unknown": 1.0}
	}
	for _, s := range generalKeyValueStrings {
		if inferStringDataFormat(s, ",", ":") != kv {
			t.Errorf("%s is not matched", s)
		}
	}
	kvstr := "k::1.0|b::3.3|s::4.32"
	if inferStringDataFormat(kvstr, "|", "::") != kv {
		t.Errorf("%s is not matched", kvstr)
	}
	nonKeyValueStrings := []string{
		"100",
		"-10:100",
		"10.2:23,",
		"0:abc",
	}
	for _, s := range nonKeyValueStrings {
		if inferStringDataFormat(s, "", "") == kv {
			t.Errorf("%s should not be matched", s)
		}
	}
}

func TestGetMaxIndexOfKeyValueString(t *testing.T) {
	a := assert.New(t)

	index, err := getMaxIndexOfKeyValueData("1:2 3:4 2:1")
	a.NoError(err)
	a.Equal(3, index)

	index, err = getMaxIndexOfKeyValueData("7:2\t 3:-4 10:20")
	a.NoError(err)
	a.Equal(10, index)

	index, err = getMaxIndexOfKeyValueData("1:2")
	a.NoError(err)
	a.Equal(1, index)
}

func mockTrainStmtNormal() *TrainStmt {
	attrs := make(map[string]interface{})
	attrs["model.nclasses"] = 2
	features := make(map[string][]FeatureColumn)
	features["feature_columns"] = []FeatureColumn{
		&EmbeddingColumn{CategoryColumn: nil, Dimension: 128, Combiner: "sum", Name: "c3"},
		&EmbeddingColumn{
			CategoryColumn: &CategoryIDColumn{
				FieldDesc:  &FieldDesc{Name: "c5", DType: Int, Shape: []int{10000}, Delimiter: ",", IsSparse: true},
				BucketSize: 10000,
			},
			Dimension: 128,
			Combiner:  "sum",
			Name:      "c5"},
	}
	label := &NumericColumn{FieldDesc: &FieldDesc{Name: "class", DType: Int, Shape: []int{1}, Delimiter: "", IsSparse: false}}

	return &TrainStmt{
		OriginalSQL: `select c1, c2, c3, c4, c5, c6, class from feature_derivation_case.train
TO TRAIN DNNClassifier
WITH model.n_classes=2
COLUMN EMBEDDING(c3, 128, sum), EMBEDDING(SPARSE(c5, 10000, COMMA), 128, sum), INDICATOR(c3)
LABEL class INTO model_table;`,
		Select:           "select c1, c2, c3, c4, c5, c6, class from feature_derivation_case.train",
		ValidationSelect: "",
		ModelImage:       "",
		Estimator:        "tf.estimator.DNNClassifier",
		Attributes:       attrs,
		Features:         features,
		Label:            label,
		Into:             "model_table",
	}
}

func mockTrainStmtCross() *TrainStmt {
	attrs := make(map[string]interface{})
	attrs["model.nclasses"] = 2
	features := make(map[string][]FeatureColumn)
	c1 := &NumericColumn{
		FieldDesc: &FieldDesc{Name: "c1", DType: Int, Shape: []int{1}, Delimiter: "", IsSparse: false},
	}
	c2 := &NumericColumn{
		FieldDesc: &FieldDesc{Name: "c2", DType: Int, Shape: []int{1}, Delimiter: "", IsSparse: false},
	}
	c4 := &NumericColumn{
		FieldDesc: &FieldDesc{Name: "c4", DType: Int, Shape: []int{1}, Delimiter: "", IsSparse: false},
	}
	c5 := &NumericColumn{
		FieldDesc: &FieldDesc{Name: "c5", DType: Int, Shape: []int{1}, Delimiter: "", IsSparse: true},
	}

	features["feature_columns"] = []FeatureColumn{
		c1, c2,
		&CrossColumn{
			Keys: []interface{}{
				c4,
				c5,
			},
			HashBucketSize: 128,
		},
		&CrossColumn{
			Keys: []interface{}{
				c1,
				c2,
			},
			HashBucketSize: 256,
		},
	}
	label := &NumericColumn{FieldDesc: &FieldDesc{
		Name:      "class",
		DType:     Int,
		Shape:     []int{1},
		Delimiter: "",
		IsSparse:  false,
	}}

	return &TrainStmt{
		OriginalSQL: `select c1, c2, c3, c4, c5, class from feature_derivation_case.train
TO TRAIN DNNClassifier
WITH model.n_classes=2
COLUMN c1, c2, CROSS([c4, c5], 128), CROSS([c1, c2], 256)
LABEL class INTO model_table;`,
		Select:           "select c1, c2, c3, c4, c5, class from feature_derivation_case.train",
		ValidationSelect: "",
		ModelImage:       "",
		Estimator:        "tf.estimator.DNNClassifier",
		Attributes:       attrs,
		Features:         features,
		Label:            label,
		Into:             "model_table",
	}

}

func mockTrainStmtIrisNoColumnClause() *TrainStmt {
	attrs := make(map[string]interface{})
	attrs["model.nclasses"] = 3
	attrs["model.hidden_units"] = []int{10, 10}
	features := make(map[string][]FeatureColumn)
	label := &NumericColumn{FieldDesc: &FieldDesc{
		Name:      "class",
		DType:     Int,
		Shape:     []int{1},
		Delimiter: "",
		IsSparse:  false,
	}}
	return &TrainStmt{
		OriginalSQL: `select * from iris.train
TO TRAIN DNNClassifier
WITH model.n_classes=3, model.hidden_units=[10,10]
LABEL class INTO model_table;`,
		Select:           "select * from iris.train",
		ValidationSelect: "",
		ModelImage:       "",
		Estimator:        "tf.estimator.DNNClassifier",
		Attributes:       attrs,
		Features:         features,
		Label:            label,
		Into:             "model_table",
	}

}

func TestFeatureDerivation(t *testing.T) {
	testDB := os.Getenv("SQLFLOW_TEST_DB")
	if testDB != "mysql" && testDB != "hive" {
		t.Skip("skip TestFeatureDerivation for tests not using MySQL or Hive")
	}

	a := assert.New(t)
	db := database.GetTestingDBSingleton()
	var testDataSQL string
	if testDB == "mysql" {
		testDataSQL = testdata.FeatureDerivationCaseSQL
	} else if testDB == "hive" {
		testDataSQL = testdata.FeatureDerivationCaseSQLHive
	}
	if err := testdata.Popularize(db.DB, testDataSQL); err != nil {
		a.FailNow(fmt.Sprintf("%v", err))
	}

	trainStmt := mockTrainStmtNormal()
	e := InferFeatureColumns(trainStmt, db)
	a.NoError(e)

	fc1 := trainStmt.Features["feature_columns"][0]
	nc, ok := fc1.(*NumericColumn)
	a.True(ok)
	a.Equal("c1", nc.FieldDesc.Name)
	a.Equal([]int{1}, nc.FieldDesc.Shape)
	a.Equal(Float, nc.FieldDesc.DType)
	a.False(nc.FieldDesc.IsSparse)

	fc2 := trainStmt.Features["feature_columns"][1]
	nc2, ok := fc2.(*NumericColumn)
	a.True(ok)
	a.Equal("c2", nc2.FieldDesc.Name)

	fc3 := trainStmt.Features["feature_columns"][2]
	emb, ok := fc3.(*EmbeddingColumn)
	a.True(ok)
	a.NotNil(emb.CategoryColumn)
	a.Equal(128, emb.Dimension)
	a.Equal("sum", emb.Combiner)
	a.Equal("c3", emb.Name)
	cat, ok := emb.CategoryColumn.(*CategoryIDColumn)
	a.True(ok)
	a.Equal("c3", cat.FieldDesc.Name)
	a.Equal([]int{4}, cat.FieldDesc.Shape)
	a.Equal(Int, cat.FieldDesc.DType)

	fc4 := trainStmt.Features["feature_columns"][3]
	nc3, ok := fc4.(*NumericColumn)
	a.True(ok)
	a.Equal("c4", nc3.FieldDesc.Name)
	a.Equal([]int{4}, nc3.FieldDesc.Shape)
	a.Equal(Float, nc3.FieldDesc.DType)
	a.False(nc3.FieldDesc.IsSparse)

	fc5 := trainStmt.Features["feature_columns"][4]
	emb2, ok := fc5.(*EmbeddingColumn)
	a.True(ok)
	a.NotNil(emb2.CategoryColumn)
	cat2, ok := emb2.CategoryColumn.(*CategoryIDColumn)
	a.True(ok)
	a.Equal(int64(10000), cat2.BucketSize)
	a.Equal("c5", cat2.FieldDesc.Name)
	a.Equal([]int{10000}, cat2.FieldDesc.Shape)
	a.Equal(Int, cat2.FieldDesc.DType)
	a.True(cat2.FieldDesc.IsSparse)

	fc6 := trainStmt.Features["feature_columns"][5]
	cat3, ok := fc6.(*EmbeddingColumn)
	a.True(ok)
	vocab := cat3.CategoryColumn.(*CategoryIDColumn).FieldDesc.Vocabulary
	a.Equal(3, len(vocab))
	_, ok = vocab["MALE"]
	a.True(ok)
	a.Equal(int64(3), cat3.CategoryColumn.(*CategoryIDColumn).BucketSize)

	a.Equal(6, len(trainStmt.Features["feature_columns"]))

	trainStmt = mockTrainStmtCross()
	e = InferFeatureColumns(trainStmt, db)
	a.NoError(e)

	a.Equal(5, len(trainStmt.Features["feature_columns"]))

	fc1 = trainStmt.Features["feature_columns"][0]
	nc, ok = fc1.(*NumericColumn)
	a.True(ok)

	fc2 = trainStmt.Features["feature_columns"][1]
	nc, ok = fc2.(*NumericColumn)
	a.True(ok)

	fc3 = trainStmt.Features["feature_columns"][2]
	nc, ok = fc3.(*NumericColumn)
	a.True(ok)

	fc4 = trainStmt.Features["feature_columns"][3]
	cc, ok := fc4.(*CrossColumn)
	a.True(ok)
	a.Equal(int64(128), cc.HashBucketSize)
	nc4, ok := cc.Keys[0].(*NumericColumn)
	a.True(ok)
	a.Equal("c4", nc4.FieldDesc.Name)
	a.Equal(Float, nc4.FieldDesc.DType)
	nc5, ok := cc.Keys[1].(*NumericColumn)
	a.True(ok)
	a.Equal("c5", nc5.FieldDesc.Name)

	fc5 = trainStmt.Features["feature_columns"][4]
	cc, ok = fc5.(*CrossColumn)
	a.True(ok)
	a.Equal(int64(256), cc.HashBucketSize)
	nc4, ok = cc.Keys[0].(*NumericColumn)
	a.True(ok)
	a.Equal("c1", nc4.FieldDesc.Name)
	a.Equal(Float, nc4.FieldDesc.DType)
	nc5, ok = cc.Keys[1].(*NumericColumn)
	a.True(ok)
	a.Equal("c2", nc5.FieldDesc.Name)
	a.Equal(Float, nc5.FieldDesc.DType)
}

func TestFeatureDerivationNoColumnClause(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST_DB") != "" && os.Getenv("SQLFLOW_TEST_DB") != "mysql" {
		t.Skip("skip TestFeatureDerivationNoColumnClause for tests not using mysql")
	}
	a := assert.New(t)
	// Prepare feature derivation test table in MySQL.
	db, err := database.OpenAndConnectDB(database.GetTestingMySQLURL())
	if err != nil {
		a.Fail("error connect to mysql: %v", err)
	}
	defer db.Close()
	err = testdata.Popularize(db.DB, testdata.IrisSQL)
	if err != nil {
		a.Fail("error creating test data: %v", err)
	}

	trainStmt := mockTrainStmtIrisNoColumnClause()
	e := InferFeatureColumns(trainStmt, db)
	a.NoError(e)

	a.Equal(4, len(trainStmt.Features["feature_columns"]))
	fc1 := trainStmt.Features["feature_columns"][0]
	_, ok := fc1.(*NumericColumn)
	a.True(ok)
}

func TestHiveFeatureDerivation(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST_DB") != "hive" {
		t.Skip("skip TestFeatureDerivationNoColumnClause for tests not using hive")
	}
	a := assert.New(t)
	trainStmt := &TrainStmt{
		Select:           "select * from iris.train",
		ValidationSelect: "select * from iris.test",
		Estimator:        "xgboost.gbtree",
		Attributes:       map[string]interface{}{},
		Features:         map[string][]FeatureColumn{},
		Label:            &NumericColumn{&FieldDesc{"class", Int, Int, "", "", "", []int{1}, false, nil, 0}}}
	e := InferFeatureColumns(trainStmt, database.GetTestingDBSingleton())
	a.NoError(e)
	a.Equal(4, len(trainStmt.Features["feature_columns"]))
}
