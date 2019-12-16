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

//go:generate goyacc -p parser -o extended_syntax_parser.go extended_syntax_parser.y
package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testStandardSelectStmt = `
SELECT employee.age, last_name, salary
FROM   employee
WHERE
  employee.age % 10 < (salary / 10000)
  AND
  strings.Upper(last_name) = "WANG"
LIMIT  100
`
	testTrainSelect = testStandardSelectStmt + `TO TRAIN DNNClassifier
WITH
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN
  employee.name,
  bucketize(last_name, 1000),
  cross(embedding(employee.name), bucketize(last_name, 1000))
LABEL "employee.salary"
INTO sqlflow_models.my_dnn_model
`
	testMultiColumnTrainSelect = testStandardSelectStmt + `TO TRAIN DNNClassifier
WITH
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN
  employee.name,
  bucketize(last_name, 1000),
  cross(embedding(employee.name), bucketize(last_name, 1000))
COLUMN
  cross(embedding(employee.name), bucketize(last_name, 1000)) FOR C2
LABEL employee.salary
INTO sqlflow_models.my_dnn_model;
`
	testPredictSelect = testStandardSelectStmt + `TO PREDICT db.table.field
USING sqlflow_models.my_dnn_model;`

	testMaxcomputeUDFPredict = `
SELECT predict_fun(concat(",", col_1, col_2)) AS (info, score) FROM db.table
TO PREDICT db.predict_result
WITH OSS_KEY=a, OSS_ID=b
USING sqlflow_models.my_model;
	`
)

func TestStandardSelect(t *testing.T) {
	a := assert.New(t)
	r, e := newExtendedSyntaxParser().Parse(testStandardSelectStmt + ";")
	a.NoError(e)
	a.False(r.extended)
	a.Equal([]string{"employee.age", "last_name", "salary"},
		r.fields.Strings())
	a.Equal([]string{"employee"}, r.tables)
	a.Equal("100", r.limit)
	a.Equal(AND, r.where.sexp[0].typ)
	a.Equal('<', rune(r.where.sexp[1].sexp[0].typ))
	a.Equal('=', rune(r.where.sexp[2].sexp[0].typ))
	a.Equal(`employee.age % 10 < (salary / 10000) AND `+
		`strings.Upper(last_name) = "WANG"`,
		r.where.String())
}

func TestTrainParser(t *testing.T) {
	a := assert.New(t)
	// NOTE(tony): Test optional semicolon at the end of the statement
	for _, s := range []string{``, `;`} {
		r, e := newExtendedSyntaxParser().Parse(testTrainSelect + s)
		a.NoError(e)
		a.True(r.extended)
		a.True(r.train)
		a.Equal("DNNClassifier", r.estimator)
		a.Equal("[10, 20]", r.trainAttrs["hidden_units"].String())
		a.Equal("3", r.trainAttrs["n_classes"].String())
		a.Equal(`employee.name`,
			r.columns["feature_columns"][0].String())
		a.Equal(`bucketize(last_name, 1000)`,
			r.columns["feature_columns"][1].String())
		a.Equal(
			`cross(embedding(employee.name), bucketize(last_name, 1000))`,
			r.columns["feature_columns"][2].String())
		a.Equal("employee.salary", r.label)
		a.Equal("sqlflow_models.my_dnn_model", r.save)
	}
}

func TestMultiColumnTrainParser(t *testing.T) {
	a := assert.New(t)
	r, e := newExtendedSyntaxParser().Parse(testMultiColumnTrainSelect)
	a.NoError(e)
	a.True(r.extended)
	a.True(r.train)
	a.Equal("DNNClassifier", r.estimator)
	a.Equal("[10, 20]", r.trainAttrs["hidden_units"].String())
	a.Equal("3", r.trainAttrs["n_classes"].String())
	a.Equal(`employee.name`,
		r.columns["feature_columns"][0].String())
	a.Equal(`bucketize(last_name, 1000)`,
		r.columns["feature_columns"][1].String())
	a.Equal(
		`cross(embedding(employee.name), bucketize(last_name, 1000))`,
		r.columns["feature_columns"][2].String())
	a.Equal(
		`cross(embedding(employee.name), bucketize(last_name, 1000))`,
		r.columns["C2"][0].String())
	a.Equal("employee.salary", r.label)
	a.Equal("sqlflow_models.my_dnn_model", r.save)
}

func TestPredictParser(t *testing.T) {
	a := assert.New(t)
	r, e := newExtendedSyntaxParser().Parse(testPredictSelect)
	a.NoError(e)
	a.True(r.extended)
	a.False(r.train)
	a.Equal("sqlflow_models.my_dnn_model", r.model)
	a.Equal("db.table.field", r.into)
}

func TestAnalyzeParser(t *testing.T) {
	a := assert.New(t)
	{
		r, e := newExtendedSyntaxParser().Parse(`select * from mytable
TO EXPLAIN my_model
USING TreeExplainer;`)
		a.NoError(e)
		a.True(r.extended)
		a.False(r.train)
		a.True(r.analyze)
		a.Equal("my_model", r.trainedModel)
		a.Equal("TreeExplainer", r.explainer)
	}
	{
		r, e := newExtendedSyntaxParser().Parse(`select * from mytable
TO EXPLAIN my_model
WITH
  plots = force
USING TreeExplainer;`)
		a.NoError(e)
		a.True(r.extended)
		a.False(r.train)
		a.True(r.analyze)
		a.Equal("my_model", r.trainedModel)
		a.Equal("force", r.explainAttrs["plots"].String())
		a.Equal("TreeExplainer", r.explainer)
	}
}

func TestSelectStarAndPrint(t *testing.T) {
	a := assert.New(t)
	r, e := newExtendedSyntaxParser().Parse(`SELECT *, b FROM a LIMIT 10;`)
	a.NoError(e)
	a.Equal(2, len(r.fields.Strings()))
	a.Equal("*", r.fields.Strings()[0])
	a.False(r.extended)
	a.False(r.train)
	a.Equal("SELECT *, b\nFROM a\nLIMIT 10", r.standardSelect.String())
}

func TestStandardDropTable(t *testing.T) {
	a := assert.New(t)
	_, e := newExtendedSyntaxParser().Parse(`DROP TABLE TO PREDICT`)
	a.Error(e)
	// Note: currently, our parser doesn't accept anything statements other than SELECT.
	// It will support parsing any SQL statements and even dialects in the future.
}

func TestDuplicatedFrom(t *testing.T) {
	a := assert.New(t)
	_, e := newExtendedSyntaxParser().Parse(`SELECT table.field FROM table FROM tttt;`)
	a.Error(e)
}

func TestSelectMaxcomputeUDF(t *testing.T) {
	a := assert.New(t)
	r, e := newExtendedSyntaxParser().Parse(testMaxcomputeUDFPredict)
	a.NoError(e)
	a.Equal(3, len(r.fields.Strings()))
	a.Equal(r.fields[0].String(), `predict_fun(concat(",", col_1, col_2))`)
	a.Equal(r.fields[1].String(), `AS`)
	a.Equal(r.fields[2].String(), `(info, score)`)
	a.Equal(r.predictClause.into, "db.predict_result")
	a.Equal(r.predAttrs["OSS_KEY"].String(), "a")
	a.Equal(r.predAttrs["OSS_ID"].String(), "b")
	a.Equal(r.predictClause.model, "sqlflow_models.my_model")
}
