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
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/pkg/sql/ir"
	"sqlflow.org/sqlflow/pkg/sql/testdata"
)

func TestCSVRegex(t *testing.T) {
	csvRegex, err := regexp.Compile("(\\-?[0-9\\.]\\,)+(\\-?[0-9\\.])")
	if err != nil {
		t.Errorf("%v", err)
	}
	csvStings := []string{
		"1,2,3,4",
		"1.3,-3.2,132,32",
		"33,-33",
	}
	for _, s := range csvStings {
		if !csvRegex.MatchString(s) {
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
		if csvRegex.MatchString(s) {
			t.Errorf("%s should not be matched", s)
		}
	}
}

func TestFeatureDerivation(t *testing.T) {
	a := assert.New(t)
	// Prepare feature derivation test table in MySQL.
	db, err := NewDB("mysql://root:root@tcp/?maxAllowedPacket=0")
	if err != nil {
		a.Fail("error connect to mysql: %v", err)
	}
	err = testdata.Popularize(db.DB, testdata.FeatureDerivationCaseSQL)
	if err != nil {
		a.Fail("error creating test data: %v", err)
	}

	parser := newExtendedSyntaxParser()

	normal := `select c1, c2, c3, c4, c5, c6, class from feature_derivation_case.train
	TO TRAIN DNNClassifier
	WITH model.n_classes=2
	COLUMN EMBEDDING(c3, 128, sum), EMBEDDING(SPARSE(c5, 10000, COMMA), 128, sum)
	LABEL class INTO model_table;`

	r, e := parser.Parse(normal)
	a.NoError(e)
	trainStmt, e := generateTrainStmt(r, "mysql://root:root@tcp/?maxAllowedPacket=0")
	a.NoError(e)
	e = InferFeatureColumns(trainStmt)
	a.NoError(e)

	fc1 := trainStmt.Features["feature_columns"][0]
	nc, ok := fc1.(*ir.NumericColumn)
	a.True(ok)
	a.Equal("c1", nc.FieldMeta.Name)
	a.Equal([]int{1}, nc.FieldMeta.Shape)
	a.Equal(ir.Float, nc.FieldMeta.DType)
	a.False(nc.FieldMeta.IsSparse)

	fc2 := trainStmt.Features["feature_columns"][1]
	nc2, ok := fc2.(*ir.NumericColumn)
	a.True(ok)
	a.Equal("c2", nc2.FieldMeta.Name)

	fc3 := trainStmt.Features["feature_columns"][2]
	emb, ok := fc3.(*ir.EmbeddingColumn)
	a.True(ok)
	a.NotNil(emb.CategoryColumn)
	a.Equal(128, emb.Dimension)
	a.Equal("sum", emb.Combiner)
	a.Equal("c3", emb.Name)
	cat, ok := emb.CategoryColumn.(*ir.CategoryIDColumn)
	a.True(ok)
	a.Equal("c3", cat.FieldMeta.Name)
	a.Equal([]int{4}, cat.FieldMeta.Shape)
	a.Equal(ir.Int, cat.FieldMeta.DType)

	fc4 := trainStmt.Features["feature_columns"][3]
	nc3, ok := fc4.(*ir.NumericColumn)
	a.True(ok)
	a.Equal("c4", nc3.FieldMeta.Name)
	a.Equal([]int{4}, nc3.FieldMeta.Shape)
	a.Equal(ir.Float, nc3.FieldMeta.DType)
	a.False(nc3.FieldMeta.IsSparse)

	fc5 := trainStmt.Features["feature_columns"][4]
	emb2, ok := fc5.(*ir.EmbeddingColumn)
	a.True(ok)
	a.NotNil(emb2.CategoryColumn)
	cat2, ok := emb2.CategoryColumn.(*ir.CategoryIDColumn)
	a.True(ok)
	a.Equal(int64(10000), cat2.BucketSize)
	a.Equal("c5", cat2.FieldMeta.Name)
	a.Equal([]int{10000}, cat2.FieldMeta.Shape)
	a.Equal(ir.Int, cat2.FieldMeta.DType)
	a.True(cat2.FieldMeta.IsSparse)

	fc6 := trainStmt.Features["feature_columns"][5]
	cat3, ok := fc6.(*ir.CategoryIDColumn)
	a.True(ok)
	a.Equal(3, len(cat3.FieldMeta.Vocabulary))
	_, ok = cat3.FieldMeta.Vocabulary["MALE"]
	a.True(ok)
	a.Equal(int64(3), cat3.BucketSize)

	a.Equal(6, len(trainStmt.Features["feature_columns"]))

	crossSQL := `select c1, c2, c3, class from feature_derivation_case.train
	TO TRAIN DNNClassifier
	WITH model.n_classes=2
	COLUMN c1, c2, CROSS([c1, c2], 256)
	LABEL class INTO model_table;`

	parser = newExtendedSyntaxParser()
	r, e = parser.Parse(crossSQL)
	a.NoError(e)
	trainStmt, e = generateTrainStmt(r, "mysql://root:root@tcp/?maxAllowedPacket=0")
	a.NoError(e)
	e = InferFeatureColumns(trainStmt)
	a.NoError(e)

	fc1 = trainStmt.Features["feature_columns"][0]
	nc, ok = fc1.(*ir.NumericColumn)
	a.True(ok)

	fc2 = trainStmt.Features["feature_columns"][1]
	nc, ok = fc2.(*ir.NumericColumn)
	a.True(ok)

	fc3 = trainStmt.Features["feature_columns"][2]
	nc, ok = fc3.(*ir.NumericColumn)
	a.True(ok)

	fc4 = trainStmt.Features["feature_columns"][3]
	cc, ok := fc4.(*ir.CrossColumn)
	a.True(ok)
	a.Equal(256, cc.HashBucketSize)
	nc4, ok := cc.Keys[0].(*ir.NumericColumn)
	a.True(ok)
	a.Equal("c1", nc4.FieldMeta.Name)
	a.Equal(ir.Float, nc4.FieldMeta.DType)
	nc5, ok := cc.Keys[1].(*ir.NumericColumn)
	a.True(ok)
	a.Equal("c2", nc5.FieldMeta.Name)
	a.Equal(ir.Float, nc5.FieldMeta.DType)

	a.Equal(4, len(trainStmt.Features["feature_columns"]))
}

func TestFeatureDerivationNoColumnClause(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST_DB") != "" && os.Getenv("SQLFLOW_TEST_DB") != "mysql" {
		t.Skip("skip TestFeatureDerivationNoColumnClause for tests not using mysql")
	}
	a := assert.New(t)
	// Prepare feature derivation test table in MySQL.
	db, err := NewDB("mysql://root:root@tcp/?maxAllowedPacket=0")
	if err != nil {
		a.Fail("error connect to mysql: %v", err)
	}
	err = testdata.Popularize(db.DB, testdata.IrisSQL)
	if err != nil {
		a.Fail("error creating test data: %v", err)
	}

	parser := newExtendedSyntaxParser()

	normal := `select * from iris.train
	TO TRAIN DNNClassifier
	WITH model.n_classes=3, model.hidden_units=[10,10]
	LABEL class INTO model_table;`

	r, e := parser.Parse(normal)
	a.NoError(e)
	trainStmt, e := generateTrainStmt(r, "mysql://root:root@tcp/?maxAllowedPacket=0")
	a.NoError(e)
	e = InferFeatureColumns(trainStmt)
	a.NoError(e)

	a.Equal(4, len(trainStmt.Features["feature_columns"]))
	fc1 := trainStmt.Features["feature_columns"][0]
	_, ok := fc1.(*ir.NumericColumn)
	a.True(ok)
}

func TestHiveFeatureDerivation(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST_DB") != "hive" {
		t.Skip("skip TestFeatureDerivationNoColumnClause for tests not using hive")
	}
	a := assert.New(t)
	trainStmt := &ir.TrainStmt{
		ExtendedSQL: ir.ExtendedSQL{
			"select * from iris.train", testDB.driverName + "://" + testDB.dataSourceName, nil, ""},
		ValidationSelect: "select * from iris.test",
		Estimator:        "xgboost.gbtree",
		Features:         map[string][]ir.FeatureColumn{},
		Label:            &ir.NumericColumn{&ir.FieldMeta{"class", ir.Int, "", []int{1}, false, nil, 0}}}
	e := InferFeatureColumns(trainStmt)
	a.NoError(e)
	a.Equal(4, len(trainStmt.Features["feature_columns"]))
}
