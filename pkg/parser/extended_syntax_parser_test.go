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

//go:generate goyacc -p extendedSyntax -o extended_syntax_parser.go extended_syntax_parser.y
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
	r, _, e := parseSQLFlowStmt(testStandardSelectStmt + ";")
	a.NoError(e)
	a.False(r.Extended)
	a.Equal([]string{"employee.age", "last_name", "salary"},
		r.Fields.Strings())
	a.Equal([]string{"employee"}, r.Tables)
	a.Equal("100", r.limit)
	a.Equal(AND, r.where.Sexp[0].Type)
	a.Equal('<', rune(r.where.Sexp[1].Sexp[0].Type))
	a.Equal('=', rune(r.where.Sexp[2].Sexp[0].Type))
	a.Equal(`employee.age % 10 < (salary / 10000) AND `+
		`strings.Upper(last_name) = "WANG"`,
		r.where.String())
}

func TestTrainParser(t *testing.T) {
	a := assert.New(t)
	// NOTE(tony): Test optional semicolon at the end of the statement
	for _, s := range []string{``, `;`} {
		r, _, e := parseSQLFlowStmt(testTrainSelect + s)
		a.NoError(e)
		a.True(r.Extended)
		a.True(r.Train)
		a.Equal("DNNClassifier", r.Estimator)
		a.Equal("[10, 20]", r.TrainAttrs["hidden_units"].String())
		a.Equal("3", r.TrainAttrs["n_classes"].String())
		a.Equal(`employee.name`,
			r.Columns["feature_columns"][0].String())
		a.Equal(`bucketize(last_name, 1000)`,
			r.Columns["feature_columns"][1].String())
		a.Equal(
			`cross(embedding(employee.name), bucketize(last_name, 1000))`,
			r.Columns["feature_columns"][2].String())
		a.Equal("employee.salary", r.Label)
		a.Equal("sqlflow_models.my_dnn_model", r.Save)
	}
}

func TestMultiColumnTrainParser(t *testing.T) {
	a := assert.New(t)
	r, _, e := parseSQLFlowStmt(testMultiColumnTrainSelect)
	a.NoError(e)
	a.True(r.Extended)
	a.True(r.Train)
	a.Equal("DNNClassifier", r.Estimator)
	a.Equal("[10, 20]", r.TrainAttrs["hidden_units"].String())
	a.Equal("3", r.TrainAttrs["n_classes"].String())
	a.Equal(`employee.name`,
		r.Columns["feature_columns"][0].String())
	a.Equal(`bucketize(last_name, 1000)`,
		r.Columns["feature_columns"][1].String())
	a.Equal(
		`cross(embedding(employee.name), bucketize(last_name, 1000))`,
		r.Columns["feature_columns"][2].String())
	a.Equal(
		`cross(embedding(employee.name), bucketize(last_name, 1000))`,
		r.Columns["C2"][0].String())
	a.Equal("employee.salary", r.Label)
	a.Equal("sqlflow_models.my_dnn_model", r.Save)
}

func TestPredictParser(t *testing.T) {
	a := assert.New(t)
	r, _, e := parseSQLFlowStmt(testPredictSelect)
	a.NoError(e)
	a.True(r.Extended)
	a.False(r.Train)
	a.Equal("sqlflow_models.my_dnn_model", r.Model)
	a.Equal("db.table.field", r.Into)
}

func TestExplainParser(t *testing.T) {
	a := assert.New(t)
	{
		r, _, e := parseSQLFlowStmt(`select * from mytable
TO EXPLAIN my_model
USING TreeExplainer;`)
		a.NoError(e)
		a.True(r.Extended)
		a.False(r.Train)
		a.True(r.Explain)
		a.Equal("my_model", r.TrainedModel)
		a.Equal("TreeExplainer", r.Explainer)
	}
	{
		r, _, e := parseSQLFlowStmt(`select * from mytable
TO EXPLAIN my_model
WITH
  plots = force
USING TreeExplainer;`)
		a.NoError(e)
		a.True(r.Extended)
		a.False(r.Train)
		a.True(r.Explain)
		a.Equal("my_model", r.TrainedModel)
		a.Equal("force", r.ExplainAttrs["plots"].String())
		a.Equal("TreeExplainer", r.Explainer)
	}
}

func TestSelectStarAndPrint(t *testing.T) {
	a := assert.New(t)
	r, _, e := parseSQLFlowStmt(`SELECT *, b FROM a LIMIT 10;`)
	a.NoError(e)
	a.Equal(2, len(r.Fields.Strings()))
	a.Equal("*", r.Fields.Strings()[0])
	a.False(r.Extended)
	a.False(r.Train)
	a.Equal("SELECT *, b\nFROM a\nLIMIT 10", r.StandardSelect.String())
}

func TestStandardDropTable(t *testing.T) {
	a := assert.New(t)
	_, _, e := parseSQLFlowStmt(`DROP TABLE TO PREDICT`)
	a.Error(e)
	// Note: currently, our parser doesn't accept anything statements other than SELECT.
	// It will support parsing any SQL statements and even dialects in the future.
}

func TestDuplicatedFrom(t *testing.T) {
	a := assert.New(t)
	_, _, e := parseSQLFlowStmt(`SELECT table.field FROM table FROM tttt;`)
	a.Error(e)
}

func TestSelectMaxcomputeUDF(t *testing.T) {
	a := assert.New(t)
	r, _, e := parseSQLFlowStmt(testMaxcomputeUDFPredict)
	a.NoError(e)
	a.Equal(3, len(r.Fields.Strings()))
	a.Equal(r.Fields[0].String(), `predict_fun(concat(",", col_1, col_2))`)
	a.Equal(r.Fields[1].String(), `AS`)
	a.Equal(r.Fields[2].String(), `(info, score)`)
	a.Equal(r.PredictClause.Into, "db.predict_result")
	a.Equal(r.PredAttrs["OSS_KEY"].String(), "a")
	a.Equal(r.PredAttrs["OSS_ID"].String(), "b")
	a.Equal(r.PredictClause.Model, "sqlflow_models.my_model")
}
