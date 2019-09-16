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
	"fmt"
	"regexp"
	"testing"

	"github.com/sql-machine-learning/sqlflow/sql/columns"

	"github.com/sql-machine-learning/sqlflow/sql/testdata"
	"github.com/stretchr/testify/assert"
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
	err = testdata.Popularize(db.DB, testdata.FeatureDericationCaseSQL)
	if err != nil {
		a.Fail("error creating test data: %v", err)
	}

	parser := newParser()

	normal := `select c1, c2, c3, c4, c5, class from feature_derivation_case.train
	TRAIN DNNClassifier
	WITH model.n_classes=2
	COLUMN EMBEDDING(c3, 128, sum), EMBEDDING(SPARSE(c5, 10000, COMMA), 128, sum)
	LABEL class INTO model_table;`

	r, e := parser.Parse(normal)
	a.NoError(e)

	connConfig, e := newConnectionConfig(db)
	a.NoError(e)
	res, e := resolveTrainClause(&r.trainClause, &r.standardSelect, connConfig)
	a.NoError(e)

	cs := res.ColumnSpecInfered["c1"]
	a.Equal("c1", cs.ColumnName)
	a.Equal([]int{1}, cs.Shape)
	a.Equal("float32", cs.DType)
	a.False(cs.IsSparse)

	cs = res.ColumnSpecInfered["c3"]
	a.Equal("c3", cs.ColumnName)
	a.Equal([]int{4}, cs.Shape)
	a.Equal("int64", cs.DType)

	cs = res.ColumnSpecInfered["c4"]
	a.Equal("c4", cs.ColumnName)
	a.Equal([]int{4}, cs.Shape)
	a.Equal("float32", cs.DType)
	a.False(cs.IsSparse)

	cs = res.ColumnSpecInfered["c5"]
	a.Equal("c5", cs.ColumnName)
	a.Equal([]int{10000}, cs.Shape)
	a.Equal("int", cs.DType)
	a.True(cs.IsSparse)

	fmt.Printf("fc inferred: %v\n", res.FeatureColumnInfered)
	fc := res.FeatureColumnInfered["feature_columns"]["c1"]
	a.Equal(columns.ColumnTypeNumeric, fc.GetColumnType())

	fc = res.FeatureColumnInfered["feature_columns"]["c3"]
	a.Equal(columns.ColumnTypeEmbedding, fc.GetColumnType())
	emb, ok := fc.(*columns.EmbeddingColumn)
	a.True(ok)
	a.NotNil(emb.CategoryColumn)
	a.Equal("c3", emb.CategoryColumn.(*columns.CategoryIDColumn).GetKey())

	fc = res.FeatureColumnInfered["feature_columns"]["c5"]
	a.Equal(columns.ColumnTypeEmbedding, fc.GetColumnType())
	emb, ok = fc.(*columns.EmbeddingColumn)
	a.Equal(10000, emb.CategoryColumn.(*columns.CategoryIDColumn).BucketSize)
}
