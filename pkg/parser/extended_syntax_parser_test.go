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
	testToPredict = `TO PREDICT db.table.field
USING sqlflow_models.my_dnn_model;`
)

func TestExtendedSyntaxParseToTrain(t *testing.T) {
	a := assert.New(t)
	r, idx, e := parseSQLFlowStmt(testToTrain)
	a.NoError(e)
	a.Equal(len(testToTrain), idx)
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

func TestExtendedSyntaxParseToTrainWithMultiColumns(t *testing.T) {
	a := assert.New(t)
	r, idx, e := parseSQLFlowStmt(testToTrainWithMultiColumns)
	a.NoError(e)
	a.Equal(len(testToTrainWithMultiColumns), idx)
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

func TestExtendedSyntaxParseToPredict(t *testing.T) {
	a := assert.New(t)
	r, idx, e := parseSQLFlowStmt(testToPredict)
	a.NoError(e)
	a.Equal(len(testToPredict), idx)
	a.True(r.Extended)
	a.False(r.Train)
	a.Equal("sqlflow_models.my_dnn_model", r.Model)
	a.Equal("db.table.field", r.Into)
}

func TestExtendedSyntaxParseToExplain(t *testing.T) {
	a := assert.New(t)
	s := `TO EXPLAIN my_model
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

func TestExtendedSyntaxParseToExplainInto(t *testing.T) {
	a := assert.New(t)
	s := `TO EXPLAIN my_model
WITH plots = force
USING TreeExplainer
INTO db.table;`
	r, idx, e := parseSQLFlowStmt(s)
	a.Equal(len(s), idx) // right before ; due to the end_of_stmt syntax rule.
	a.NoError(e)
	a.True(r.Extended)
	a.True(r.Explain)
	a.Equal("db.table", r.ExplainInto)
	a.Equal("TreeExplainer", r.Explainer)
}

func TestExtendedSyntaxParseToExplainIntoNoWith(t *testing.T) {
	a := assert.New(t)
	s := `TO EXPLAIN my_model
USING TreeExplainer
INTO db.table;`
	r, idx, e := parseSQLFlowStmt(s)
	a.Equal(len(s), idx) // right before ; due to the end_of_stmt syntax rule.
	a.NoError(e)
	a.True(r.Extended)
	a.True(r.Explain)
	a.Equal("db.table", r.ExplainInto)
	a.Equal("TreeExplainer", r.Explainer)
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
		a.Equal(3, idx) // right before DROP as there was an error.
		a.Error(e)
	}
}
