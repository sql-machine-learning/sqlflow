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

package sql

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/test"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/pkg/ir"
	"sqlflow.org/sqlflow/pkg/parser"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

func TestGenerateTrainStmtWithTypeCheck(t *testing.T) {
	a := assert.New(t)
	wrong := "SELECT * FROM t1 TO TRAIN DNNClassifier WITH model.stddev=0.1 LABEL c INTO m;"
	r, e := parser.ParseStatement("mysql", wrong)
	a.NoError(e)
	trainStmt, err := ir.GenerateTrainStmt(r.SQLFlowSelectStmt)
	a.Error(initializeAndCheckAttributes(trainStmt))

	normal := `SELECT c1, c2, c3, c4 FROM my_table
	TO TRAIN DNNClassifier
	WITH
		model.n_classes=2,
		model.optimizer="adam",
		model.hidden_units=[128,64]
	LABEL c4
	INTO mymodel;
	`
	r, e = parser.ParseStatement("mysql", normal)
	a.NoError(e)

	trainStmt, err = ir.GenerateTrainStmt(r.SQLFlowSelectStmt)
	a.NoError(initializeAndCheckAttributes(trainStmt))
	a.NoError(err)
	a.Equal("DNNClassifier", trainStmt.Estimator)
	a.Equal("SELECT c1, c2, c3, c4 FROM my_table\n	", trainStmt.Select)
	extendedAttr := map[string]bool{
		"train.epoch":                  true,
		"train.verbose":                true,
		"train.save_checkpoints_steps": true,
		"train.log_every_n_iter":       true,
		"train.max_steps":              true,
		"validation.steps":             true,
		"validation.metrics":           true,
		"validation.start_delay_secs":  true,
		"train.batch_size":             true,
		"validation.throttle_secs":     true,
		"validation.select":            true,
	}
	a.Equal(14, len(trainStmt.Attributes))

	for key, attr := range trainStmt.Attributes {
		if key == "model.n_classes" {
			a.Equal(2, attr.(int))
		} else if key == "model.optimizer" {
			a.Equal("adam()", attr.(string))
		} else if key == "model.hidden_units" {
			l, ok := attr.([]interface{})
			a.True(ok)
			a.Equal(128, l[0].(int))
			a.Equal(64, l[1].(int))
		} else if _, ok := extendedAttr[key]; !ok {
			a.Failf("error key", key)
		}
	}

	l, ok := trainStmt.Label.(*ir.NumericColumn)
	a.True(ok)
	a.Equal("c4", l.FieldDesc.Name)

	a.Equal("mymodel", trainStmt.Into)
}

func TestGeneratePredictStmt(t *testing.T) {
	if test.GetEnv("SQLFLOW_TEST_DB", "mysql") == "hive" {
		t.Skip(fmt.Sprintf("%s: skip Hive test", test.GetEnv("SQLFLOW_TEST_DB", "mysql")))
	}
	a := assert.New(t)

	predSQL := `SELECT * FROM iris.test
TO PREDICT iris.predict.class
USING sqlflow_models.mymodel;`
	r, e := parser.ParseStatement("mysql", predSQL)
	a.NoError(e)

	// need to save a model first because predict SQL will read the train SQL
	// from saved model
	cwd, e := ioutil.TempDir("/tmp", "sqlflow_models")
	a.Nil(e)
	defer os.RemoveAll(cwd)
	modelDir := ""
	stream := RunSQLProgram(`SELECT * FROM iris.train
TO TRAIN DNNClassifier
WITH model.n_classes=3, model.hidden_units=[10,20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.mymodel;`, modelDir, &pb.Session{DbConnStr: database.GetTestingDBSingleton().URL()})
	a.True(test.GoodStream(stream.ReadAll()))

	predStmt, err := ir.GeneratePredictStmt(r.SQLFlowSelectStmt, database.GetTestingDBSingleton().URL(), modelDir, cwd, true)
	a.NoError(err)

	a.Equal("iris.predict", predStmt.ResultTable)
	a.Equal("class", predStmt.TrainStmt.Label.GetFieldDesc()[0].Name)
	a.Equal("DNNClassifier", predStmt.TrainStmt.Estimator)
	nc, ok := predStmt.TrainStmt.Features["feature_columns"][0].(*ir.NumericColumn)
	a.True(ok)
	a.Equal("sepal_length", nc.FieldDesc.Name)
	a.Equal("sqlflow_models.mymodel", predStmt.Using)
}

func TestGenerateExplainStmt(t *testing.T) {
	if test.GetEnv("SQLFLOW_TEST_DB", "mysql") != "mysql" {
		t.Skip(fmt.Sprintf("%s: skip test", test.GetEnv("SQLFLOW_TEST_DB", "mysql")))
	}
	a := assert.New(t)
	connStr := database.GetTestingMySQLURL()

	cwd, e := ioutil.TempDir("/tmp", "sqlflow_models")
	a.Nil(e)
	defer os.RemoveAll(cwd)
	modelDir := ""
	stream := RunSQLProgram(`SELECT * FROM iris.train
TO TRAIN xgboost.gbtree
WITH
	objective="multi:softprob",
	train.num_boost_round = 30,
	eta = 0.4,
	num_class = 3
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_xgboost_model;
`, modelDir, &pb.Session{DbConnStr: connStr})
	a.NoError(e)
	a.True(test.GoodStream(stream.ReadAll()))

	pr, e := parser.ParseStatement("mysql", `
	SELECT *
	FROM iris.train
	TO EXPLAIN sqlflow_models.my_xgboost_model
	WITH
	    summary.plot_type="bar",
	    summary.alpha=1,
	    summary.sort=True
	USING TreeExplainer;
	`)
	a.NoError(e)

	ExplainStmt, e := ir.GenerateExplainStmt(pr.SQLFlowSelectStmt, connStr, modelDir, cwd, true)
	a.NoError(e)
	a.Equal(ExplainStmt.Explainer, "TreeExplainer")
	a.Equal(len(ExplainStmt.Attributes), 3)
	a.Equal(ExplainStmt.Attributes["summary.sort"], true)
	a.Equal(ExplainStmt.Attributes["summary.plot_type"], "bar")
	a.Equal(ExplainStmt.Attributes["summary.alpha"], 1)

	nc, ok := ExplainStmt.TrainStmt.Features["feature_columns"][0].(*ir.NumericColumn)
	a.True(ok)
	a.Equal("sepal_length", nc.FieldDesc.Name)

	pr, e = parser.ParseStatement("mysql", `
	SELECT *
	FROM iris.train
	TO EXPLAIN sqlflow_models.my_xgboost_model
	WITH
	    summary.plot_type="bar",
	    summary.alpha=1,
	    summary.sort=True
	USING TreeExplainer
	INTO db.explain_result;
	`)
	a.NoError(e)

	ExplainIntoStmt, e := ir.GenerateExplainStmt(pr.SQLFlowSelectStmt, connStr, modelDir, cwd, true)
	a.NoError(e)
	a.Equal(ExplainIntoStmt.Explainer, "TreeExplainer")
	a.Equal(len(ExplainIntoStmt.Attributes), 3)
	a.Equal("db.explain_result", ExplainIntoStmt.Into)

	pr, e = parser.ParseStatement("mysql", `SELECT * FROM iris.train TO EXPLAIN sqlflow_models.my_xgboost_model;`)
	a.NoError(e)
	shortExplainStmt, e := ir.GenerateExplainStmt(pr.SQLFlowSelectStmt, connStr, modelDir, cwd, true)
	a.NoError(e)
	a.Equal(shortExplainStmt.Explainer, "")
	a.Equal(len(shortExplainStmt.Attributes), 0)
}

func TestGenerateOptimizeStmt(t *testing.T) {
	a := assert.New(t)

	oneVarSQL := `
SELECT * FROM alifin_jtest_dev.woodcarving
TO MAXIMIZE SUM((price - materials_cost - other_cost) * product)
CONSTRAINT SUM(finishing * product) <= 100,
           SUM(carpentry * product) <= 80,
           product <= max_num
WITH variables="product",
     var_type="NonNegativeIntegers"
USING glpk
INTO result_table;
`
	r, e := parser.Parse("mysql", oneVarSQL)
	a.NoError(e)
	a.Equal(1, len(r))
	stmt, e := ir.GenerateOptimizeStmt(r[0].SQLFlowSelectStmt)
	a.NoError(e)
	a.Equal("maximize", stmt.Direction)
	a.Equal([]string{"SUM", "(", "(", "price", "-", "materials_cost", "-", "other_cost", ")", "*", "product", ")"}, stmt.Objective.ExpressionTokens)
	a.Equal("", stmt.Objective.GroupBy)

	a.Equal(3, len(stmt.Constraints))
	a.Equal([]string{"SUM", "(", "finishing", "*", "product", ")", "<=", "100"}, stmt.Constraints[0].ExpressionTokens)
	a.Equal("", stmt.Constraints[0].GroupBy)
	a.Equal([]string{"SUM", "(", "carpentry", "*", "product", ")", "<=", "80"}, stmt.Constraints[1].ExpressionTokens)
	a.Equal("", stmt.Constraints[1].GroupBy)
	a.Equal([]string{"product", "<=", "max_num"}, stmt.Constraints[2].ExpressionTokens)
	a.Equal("", stmt.Constraints[2].GroupBy)

	a.Equal("glpk", stmt.Solver)
	a.Equal(1, len(stmt.Variables))
	a.Equal("product", stmt.Variables[0])
	a.Equal("product", stmt.ResultValueName)
	a.Equal("NonNegativeIntegers", stmt.VariableType)
	a.Equal("result_table", stmt.ResultTable)

	oneVarSQLWithResultValueName := `
SELECT * FROM alifin_jtest_dev.woodcarving
TO MAXIMIZE SUM((price - materials_cost - other_cost) * amount)
CONSTRAINT SUM(finishing * amount) <= 100,
           SUM(carpentry * amount) <= 80,
           product <= max_num
WITH variables="amount(product)",
     var_type="NonNegativeIntegers"
USING glpk
INTO result_table;
`
	r, e = parser.Parse("mysql", oneVarSQLWithResultValueName)
	a.NoError(e)
	stmt, e = ir.GenerateOptimizeStmt(r[0].SQLFlowSelectStmt)
	a.NoError(e)
	a.Equal("amount", stmt.ResultValueName)

	twoVarSQL := `
SELECT * FROM alifin_jtest_dev.zjl_shipment_test
TO MINIMIZE SUM(distance * shipment * 90 / 1000)
CONSTRAINT SUM(shipment) <= capacity GROUP BY plants,
           SUM(shipment) >= demand GROUP BY markets
WITH variables = "shipment(plants,markets)",
     var_type = "NonNegativeReals",
     data.enable_slice = True,
     data.batch_size = 1,
     worker.core = 16,
     worker.num = 4,
     worker.memory = 8192,
     solver.max_iter = 10
USING glpk
INTO shipment_result_table;
`
	r, e = parser.Parse("mysql", twoVarSQL)
	a.NoError(e)
	a.Equal(1, len(r))
	stmt, e = ir.GenerateOptimizeStmt(r[0].SQLFlowSelectStmt)
	a.NoError(e)
	a.Equal("minimize", stmt.Direction)
	a.Equal([]string{"SUM", "(", "distance", "*", "shipment", "*", "90", "/", "1000", ")"}, stmt.Objective.ExpressionTokens)
	a.Equal("", stmt.Objective.GroupBy)

	a.Equal(2, len(stmt.Constraints))
	a.Equal([]string{"SUM", "(", "shipment", ")", "<=", "capacity"}, stmt.Constraints[0].ExpressionTokens)
	a.Equal("plants", stmt.Constraints[0].GroupBy)
	a.Equal([]string{"SUM", "(", "shipment", ")", ">=", "demand"}, stmt.Constraints[1].ExpressionTokens)
	a.Equal("markets", stmt.Constraints[1].GroupBy)

	a.Equal("glpk", stmt.Solver)
	a.Equal(2, len(stmt.Variables))
	a.Equal("plants", stmt.Variables[0])
	a.Equal("markets", stmt.Variables[1])
	a.Equal("shipment", stmt.ResultValueName)
	a.Equal("NonNegativeReals", stmt.VariableType)
	a.Equal("shipment_result_table", stmt.ResultTable)

	a.Equal(true, stmt.Attributes["data.enable_slice"])
	a.Equal(1, stmt.Attributes["data.batch_size"])
	a.Equal(16, stmt.Attributes["worker.core"])
	a.Equal(4, stmt.Attributes["worker.num"])
	a.Equal(8192, stmt.Attributes["worker.memory"])
	a.Equal(10, stmt.Attributes["solver.max_iter"])
}
