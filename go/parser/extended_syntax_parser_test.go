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

//go:generate goyacc -p extendedSyntax -o extended_syntax_parser.go extended_syntax_parser.y
package parser

import (
	"reflect"
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
  cross(embedding(employee.name), bucketize(last_name, 1000)),
  cross(indicator(employee.name), bucketize(last_name, 1000))
LABEL "employee.salary"
INTO sqlflow_models.my_dnn_model;
`
	testToTrainWithMultiColumns = `TO TRAIN DNNClassifier
WITH
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN
	employee.name,
  bucketize(last_name, 1000),
  cross(embedding(employee.name), bucketize(last_name, 1000)),
	cross(indicator(employee.name), bucketize(last_name, 1000)),
	concat(",", employee.col1, employee.col2),
	concat(',', employee.col1, employee.col2)
COLUMN
  cross(embedding(employee.name), bucketize(last_name, 1000)),
  cross(indicator(employee.name), bucketize(last_name, 1000)) FOR C2
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
	a.Equal(
		`cross(indicator(employee.name), bucketize(last_name, 1000))`,
		r.Columns["feature_columns"][3].String())

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
		`cross(indicator(employee.name), bucketize(last_name, 1000))`,
		r.Columns["feature_columns"][3].String())
	// test function argument with double quotes
	a.Equal(
		`concat(",", employee.col1, employee.col2)`,
		r.Columns["feature_columns"][4].String())
	// test function argument with single quotes
	a.Equal(
		`concat(',', employee.col1, employee.col2)`,
		r.Columns["feature_columns"][5].String())
	a.Equal(
		`cross(embedding(employee.name), bucketize(last_name, 1000))`,
		r.Columns["C2"][0].String())
	a.Equal(
		`cross(indicator(employee.name), bucketize(last_name, 1000))`,
		r.Columns["C2"][1].String())
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

func TestExtendedSyntaxParseToEvaluate(t *testing.T) {
	a := assert.New(t)
	s := `TO EVALUATE my_model WITH validation.metrics="MAE,MSE" LABEL class INTO evaluation_result;`
	r, idx, e := parseSQLFlowStmt(s)
	a.NoError(e)
	a.Equal(len(s), idx)
	a.True(r.Extended)
	a.False(r.Train)
	a.False(r.Predict)
	a.True(r.Evaluate)
	a.Equal("my_model", r.ModelToEvaluate)
	a.Equal("\"MAE,MSE\"", r.EvaluateAttrs["validation.metrics"].String())
	a.Equal("class", r.EvaluateLabel)
	a.Equal("evaluation_result", r.EvaluateInto)
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

func TestExtendedSyntaxParseUnmatchedQuotation(t *testing.T) {
	a := assert.New(t)
	{
		// unmatched quotation within a statement
		r, idx, e := parseSQLFlowStmt(`TO TRAIN "some_thing`)
		a.Error(e)
		a.Equal(9, idx)
		a.Nil(r)
	}
	{
		// unmatched quotation right after the statement
		_, idx, e := parseSQLFlowStmt(`to train a with b = c label d into e;"`)
		a.Error(e)
		a.Equal(len(`to train a with b = c label d into e;`), idx)
	}

}

func TestExtendedSyntaxOptimize(t *testing.T) {
	a := assert.New(t)
	s := `TO MAXIMIZE SUM((price - materials_cost - other_cost) * product)
CONSTRAINT SUM(finishing * product) <= 100,
           SUM(carpentry * product) <= 80,
		   product <= max_num
WITH variables="product",
	 product="Integers"
USING glpk
INTO db.table;`
	r, idx, e := parseSQLFlowStmt(s)
	if e != nil {
		a.FailNow("%v", e)
	}
	a.Equal(len(s), idx)
	a.True(r.Extended)
	a.True(r.Optimize)
	a.Equal("MAXIMIZE", r.Direction)
	a.Equal("SUM((price - materials_cost - other_cost) * product)", r.Objective.String())
	a.Equal("SUM(finishing * product) <= 100", r.Constraints[0].String())
	a.Equal("db.table", r.OptimizeInto)
	a.Equal("glpk", r.Solver)

	s = `TO MINIMIZE SUM((price - materials_cost - other_cost) * product)
CONSTRAINT SUM(finishing * product) <= 100 GROUP BY product,
           SUM(carpentry * product) <= 80,
		   product <= max_num
WITH variables="product",
	 product="Integers"
INTO db.table;`
	r, idx, e = parseSQLFlowStmt(s)
	a.NoError(e)
	a.Equal("MINIMIZE", r.Direction)
	a.Equal("db.table", r.OptimizeInto)
	a.Equal("product", r.Constraints[0].GroupBy)
	a.Equal("", r.Solver)
}

func TestExtendedShowTrainStmt(t *testing.T) {
	a := assert.New(t)
	{
		testShowTrain := `SHOW TRAIN my_dnn_classifier_model;`
		r, idx, e := parseSQLFlowStmt(testShowTrain)
		a.Equal(nil, e)
		a.True(r.ShowTrain)
		a.True(r.Extended)
		a.NotNil(r.ShowTrainClause)
		a.Equal(`my_dnn_classifier_model`, r.ShowTrainClause.ModelName)
		a.Equal(len(testShowTrain), idx)
	}
	{
		testShowTrain := `SHOW TRAIN my_dnn_classifier_model`
		r, idx, e := parseSQLFlowStmt(testShowTrain + " bad;")
		a.Nil(r)
		a.NotNil(e)
		a.Equal(len(testShowTrain)+1, idx)
	}
	{
		testShowTrain := `SHOW TRAIN ;`
		//                           ^ err here
		r, idx, e := parseSQLFlowStmt(testShowTrain)
		a.Nil(r)
		a.NotNil(e)
		a.Equal(11, idx)
	}
}

func TestExtendedSyntaxParseToRun(t *testing.T) {
	a := assert.New(t)
	{
		testToRun := `TO RUN a_data_scientist/ts_data_processor:1.0;`
		r, idx, e := parseSQLFlowStmt(testToRun)
		a.NoError(e)
		a.True(r.Extended)
		a.True(r.Run)
		a.Equal(`a_data_scientist/ts_data_processor:1.0`, r.ImageName)
		a.Equal(len(r.Parameters), 0)
		a.Equal(len(r.OutputTables), 0)
		a.Equal(len(testToRun), idx)
	}

	{
		testToRun := `TO RUN a_data_scientist/ts_data_processor:1.0
CMD "slide_window_to_row";`
		r, idx, e := parseSQLFlowStmt(testToRun)
		a.NoError(e)
		a.True(r.Run)
		a.True(reflect.DeepEqual(r.Parameters, []string{`slide_window_to_row`}))
		a.Equal(len(r.OutputTables), 0)
		a.Equal(len(testToRun), idx)
	}

	{
		testToRun := `TO RUN a_data_scientist/ts_data_processor:1.0
CMD "slide_window_to_row"
INTO output_table;`
		r, idx, e := parseSQLFlowStmt(testToRun)
		a.NoError(e)
		a.True(r.Run)
		a.True(reflect.DeepEqual(
			r.Parameters,
			[]string{`slide_window_to_row`}))
		a.True(reflect.DeepEqual(r.OutputTables, []string{`output_table`}))
		a.Equal(len(testToRun), idx)
	}

	{
		testToRun := `TO RUN a_data_scientist/ts_data_processor:1.0
CMD "slide_window_to_row"
INTO output_table_1, output_table_2;`
		r, idx, e := parseSQLFlowStmt(testToRun)
		a.NoError(e)
		a.True(r.Run)
		a.True(reflect.DeepEqual(
			r.Parameters,
			[]string{`slide_window_to_row`}))
		a.True(reflect.DeepEqual(
			r.OutputTables,
			[]string{`output_table_1`, `output_table_2`}))
		a.Equal(len(testToRun), idx)
	}

	{
		testToRun := `TO RUN a_data_scientist/ts_data_processor:1.0
CMD "slide_window_to_row", "--param_a=value_a", "--param_b=value_b"
INTO output_table_1, output_table_2;`
		r, idx, e := parseSQLFlowStmt(testToRun)
		a.NoError(e)
		a.True(r.Run)
		a.True(reflect.DeepEqual(
			r.Parameters,
			[]string{
				`slide_window_to_row`,
				`--param_a=value_a`,
				`--param_b=value_b`,
			}))
		a.True(reflect.DeepEqual(
			r.OutputTables,
			[]string{`output_table_1`, `output_table_2`}))
		a.Equal(len(testToRun), idx)
	}
}

func TestExtendedSyntaxParseToRunInvalid(t *testing.T) {
	a := assert.New(t)
	{
		testToRun := `TO RUN "a_data_scientist/ts_data_processor:1.0";`
		r, idx, e := parseSQLFlowStmt(testToRun)
		a.Nil(r)
		a.Equal(7, idx)
		a.Error(e)
	}

	{
		testToRun := `TO RUN a_data_scientist/ts_data_processor:1.0
CMD slide_window_to_row;`
		r, idx, e := parseSQLFlowStmt(testToRun)
		a.Nil(r)
		a.Equal(50, idx)
		a.Error(e)
	}

	{
		testToRun := `TO RUN a_data_scientist/ts_data_processor:1.0
CMD "slide_window_to_row"
INTO "output_table_1";`
		r, idx, e := parseSQLFlowStmt(testToRun)
		a.Nil(r)
		a.Equal(77, idx)
		a.Error(e)
	}
}
