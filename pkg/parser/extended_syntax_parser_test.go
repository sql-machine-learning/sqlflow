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
	testStandardSelect = `
SELECT employee.age, last_name, salary
FROM   employee
WHERE
  employee.age % 10 < (salary / 10000)
  AND
  strings.Upper(last_name) = "WANG"
LIMIT  100
`
	testToTrain = `TO TRAIN DNNClassifier
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
	testSelectToTrain           = testStandardSelect + testToTrain
	testToTrainWithMultiColumns = `TO TRAIN DNNClassifier
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
	testSelectToTrainWithMultiColumns = testStandardSelect + testToTrainWithMultiColumns
	testToPredict                     = `TO PREDICT db.table.field
USING sqlflow_models.my_dnn_model;`
	testSelectToPredict = testStandardSelect + testToPredict
)

// TODO(wangkuiyi): Remove this test after we remove the rules to
// parse "standard" select.
func TestExtendedSyntaxParseStandardSelect(t *testing.T) {
	a := assert.New(t)
	r, _, e := parseSQLFlowStmt(testStandardSelect + ";")
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

// TODO(wangkuiyi): Remove this test after we remove the rules to
// parse "standard" select.
func TestExtendedSyntaxParseSelectToTrain(t *testing.T) {
	a := assert.New(t)
	// NOTE(tony): Test optional semicolon at the end of the statement
	for _, s := range []string{``, `;`} {
		r, _, e := parseSQLFlowStmt(testSelectToTrain + s)
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

func TestExtendedSyntaxParseToTrain(t *testing.T) {
	a := assert.New(t)
	for _, eos := range []string{``, `;`} {
		r, e := parseSQLFlowStmt(testToTrain + eos)
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

// TODO(wangkuiyi): Remove this test after we remove the rules to
// parse "standard" select.
func TestExtendedSyntaxParseSelectToTrainWithMultiColumns(t *testing.T) {
	a := assert.New(t)
	r, _, e := parseSQLFlowStmt(testSelectToTrainWithMultiColumns)
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

func TestExtendedSyntaxParseToTrainWithMultiColumns(t *testing.T) {
	a := assert.New(t)
	r, e := parseSQLFlowStmt(testToTrainWithMultiColumns)
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

// TODO(wangkuiyi): Remove this test after we remove the rules to
// parse "standard" select.
func TestExtendedSyntaxParseSelectToPredict(t *testing.T) {
	a := assert.New(t)
	r, _, e := parseSQLFlowStmt(testSelectToPredict)
	a.NoError(e)
	a.True(r.Extended)
	a.False(r.Train)
	a.Equal("sqlflow_models.my_dnn_model", r.Model)
	a.Equal("db.table.field", r.Into)
}

func TestExtendedSyntaxParseToPredict(t *testing.T) {
	a := assert.New(t)
	r, e := parseSQLFlowStmt(testToPredict)
	a.NoError(e)
	a.True(r.Extended)
	a.False(r.Train)
	a.Equal("sqlflow_models.my_dnn_model", r.Model)
	a.Equal("db.table.field", r.Into)
}

// TODO(wangkuiyi): Remove this test after we remove the rules to
// parse "standard" select.
func TestExtendedSyntaxParseSelectToExplain(t *testing.T) {
	a := assert.New(t)
	s := `select * from mytable
TO EXPLAIN my_model
WITH
  plots = force
USING TreeExplainer;`
	r, idx, e := parseSQLFlowStmt(s)
	a.NoError(e)
	a.Equal(len(s), idx)
	a.True(r.Extended)
	a.False(r.Train)
	a.True(r.Explain)
	a.Equal("my_model", r.TrainedModel)
	a.Equal("force", r.ExplainAttrs["plots"].String())
	a.Equal("TreeExplainer", r.Explainer)
}

func TestExtendedSyntaxParseToExplain(t *testing.T) {
	a := assert.New(t)
	s := `TO EXPLAIN my_model
WITH plots = force
USING TreeExplainer;`
	r, idx, e := parseSQLFlowStmt(s)
	a.Equal(len(s), idx) // right before ; due to the end_of_stmt syntax rule.
	a.NoError(e)
	a.True(r.Extended)
	a.False(r.Train)
	a.True(r.Explain)
	a.Equal("my_model", r.TrainedModel)
	a.Equal("force", r.ExplainAttrs["plots"].String())
	a.Equal("TreeExplainer", r.Explainer)
}

func TestExtendedSyntaxParseSelectStarAndPrint(t *testing.T) {
	a := assert.New(t)
	r, idx, e := parseSQLFlowStmt(`SELECT *, b FROM a LIMIT 10  ;  `)
	a.Equal(29, idx) // right before ; due to the end_of_stmt syntax rule.
	a.NoError(e)
	a.Equal(2, len(r.Fields.Strings()))
	a.Equal("*", r.Fields.Strings()[0])
	a.False(r.Extended)
	a.False(r.Train)
	a.Equal("SELECT *, b\nFROM a\nLIMIT 10", r.StandardSelect.String())
}

func TestExtendedSyntaxParseNonSelectStmt(t *testing.T) {
	a := assert.New(t)
	{
		r, idx, e := parseSQLFlowStmt(`DROP TABLE TO PREDICT`)
		a.Nil(r)
		a.Equal(0, idx)
		a.Error(e)
	}
	{
		r, idx, e := parseSQLFlowStmt(`   DROP TABLE TO PREDICT`)
		a.Nil(r)
		a.Equal(0, idx)
		a.Error(e)
	}
}

func TestExtendedSyntaxParseSelectWithDuplicatedFromClauses(t *testing.T) {
	a := assert.New(t)
	r, idx, e := parseSQLFlowStmt(`SELECT table.field FROM table   FROM tttt;`)
	a.Error(e)
	// TODO(wangkuiyi): After removing the syntax rules parsing
	// the "standard" SELECT prefix, we will need to adjust the
	// following values.
	a.Equal(32, idx)
	a.False(r.Extended)
	a.False(r.Train)
	a.False(r.Explain)
	a.NotNil(r.StandardSelect)
}

func TestExtendedSyntaxParseSelectToPredictWithMaxcomputeUDF(t *testing.T) {
	a := assert.New(t)
	testSelectToPredictWithMaxComputeUDF := `
SELECT predict_fun(concat(",", col_1, col_2)) AS (info, score) FROM db.table
TO PREDICT db.predict_result
WITH OSS_KEY=a, OSS_ID=b
USING sqlflow_models.my_model;
	`
	r, idx, e := parseSQLFlowStmt(testSelectToPredictWithMaxComputeUDF)
	a.NoError(e)
	a.Equal(len(testSelectToPredictWithMaxComputeUDF), idx)
	a.Equal(3, len(r.Fields.Strings()))
	a.Equal(r.Fields[0].String(), `predict_fun(concat(",", col_1, col_2))`)
	a.Equal(r.Fields[1].String(), `AS`)
	a.Equal(r.Fields[2].String(), `(info, score)`)
	a.Equal(r.PredictClause.Into, "db.predict_result")
	a.Equal(r.PredAttrs["OSS_KEY"].String(), "a")
	a.Equal(r.PredAttrs["OSS_ID"].String(), "b")
	a.Equal(r.PredictClause.Model, "sqlflow_models.my_model")
}
