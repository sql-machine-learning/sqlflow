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
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/model"
	"sqlflow.org/sqlflow/go/parser"
	"sqlflow.org/sqlflow/go/test"
)

func TestGenerateTrainStmt(t *testing.T) {
	a := assert.New(t)
	normal := `SELECT c1, c2, c3, c4 FROM my_table
	TO TRAIN DNNClassifier
	WITH
		model.n_classes=2,
		train.optimizer="adam",
		model.hidden_units=[128,64],
		validation.select="SELECT c1, c2, c3, c4 FROM my_table LIMIT 10"
	COLUMN c1,DENSE(c2, [128, 32]),CATEGORY_ID(c3, 512),
		SEQ_CATEGORY_ID(c3, 512),
		CROSS([c1,c2], 64),
		BUCKET(DENSE(c1, [100]), 100),
		EMBEDDING(CATEGORY_ID(c3, 512), 128, mean),
		DENSE(c1, 64, COMMA),
		CATEGORY_ID(SPARSE(c2, 10000, COMMA), 128),
		SEQ_CATEGORY_ID(SPARSE(c2, 10000, COMMA), 128),
		EMBEDDING(c1, 128, sum),
		EMBEDDING(SPARSE(c2, 10000, COMMA, "int"), 128, sum),
		INDICATOR(CATEGORY_ID(c3, 512)),
		INDICATOR(c1),
		INDICATOR(SPARSE(c2, 10000, COMMA, "int")),
		WEIGHTED_CATEGORY(CATEGORY_ID(SPARSE(c2, 10000, "-", "int", ":", "float"), 128)),
		EMBEDDING(SPARSE(c2, 10000, "-", "int", ":", "float"), 128, sum)
	LABEL c4
	INTO mymodel;
	`

	r, e := parser.ParseStatement("mysql", normal)
	a.NoError(e)

	trainStmt, err := GenerateTrainStmt(r.SQLFlowSelectStmt)
	a.NoError(err)
	a.Equal("DNNClassifier", trainStmt.Estimator)
	a.Equal(`SELECT c1, c2, c3, c4 FROM my_table
	`, trainStmt.Select)
	a.Equal("SELECT c1, c2, c3, c4 FROM my_table LIMIT 10", trainStmt.ValidationSelect)

	for key, attr := range trainStmt.Attributes {
		if key == "model.n_classes" {
			a.Equal(2, attr.(int))
		} else if key == "train.optimizer" {
			a.Equal("adam", attr.(string))
		} else if key == "model.stddev" {
			a.Equal(float32(0.001), attr.(float32))
		} else if key == "model.hidden_units" {
			l, ok := attr.([]interface{})
			a.True(ok)
			a.Equal(128, l[0].(int))
			a.Equal(64, l[1].(int))
		} else if key != "validation.select" {
			a.Failf("error key", key)
		}
	}

	nc, ok := trainStmt.Features["feature_columns"][0].(*NumericColumn)
	a.True(ok)
	a.Equal([]int{1}, nc.FieldDesc.Shape)

	nc, ok = trainStmt.Features["feature_columns"][1].(*NumericColumn)
	a.True(ok)
	a.Equal("c2", nc.FieldDesc.Name)
	a.Equal([]int{128, 32}, nc.FieldDesc.Shape)

	cc, ok := trainStmt.Features["feature_columns"][2].(*CategoryIDColumn)
	a.True(ok)
	a.Equal("c3", cc.FieldDesc.Name)
	a.Equal(int64(512), cc.BucketSize)

	seqcc, ok := trainStmt.Features["feature_columns"][3].(*SeqCategoryIDColumn)
	a.True(ok)
	a.Equal("c3", seqcc.FieldDesc.Name)

	cross, ok := trainStmt.Features["feature_columns"][4].(*CrossColumn)
	a.True(ok)
	a.Equal("c1", cross.Keys[0].(string))
	a.Equal("c2", cross.Keys[1].(string))
	a.Equal(int64(64), cross.HashBucketSize)

	bucket, ok := trainStmt.Features["feature_columns"][5].(*BucketColumn)
	a.True(ok)
	a.Equal(100, bucket.Boundaries[0])
	a.Equal("c1", bucket.SourceColumn.FieldDesc.Name)

	emb, ok := trainStmt.Features["feature_columns"][6].(*EmbeddingColumn)
	a.True(ok)
	a.Equal("mean", emb.Combiner)
	a.Equal(128, emb.Dimension)
	embInner, ok := emb.CategoryColumn.(*CategoryIDColumn)
	a.True(ok)
	a.Equal("c3", embInner.FieldDesc.Name)
	a.Equal(int64(512), embInner.BucketSize)

	// DENSE(c1, [64], COMMA), [128]
	nc, ok = trainStmt.Features["feature_columns"][7].(*NumericColumn)
	a.True(ok)
	a.Equal(64, nc.FieldDesc.Shape[0])
	a.Equal(",", nc.FieldDesc.Delimiter)
	a.False(nc.FieldDesc.IsSparse)

	// CATEGORY_ID(SPARSE(c2, 10000, COMMA), 128),
	cc, ok = trainStmt.Features["feature_columns"][8].(*CategoryIDColumn)
	a.True(ok)
	a.True(cc.FieldDesc.IsSparse)
	a.Equal("c2", cc.FieldDesc.Name)
	a.Equal(10000, cc.FieldDesc.Shape[0])
	a.Equal(",", cc.FieldDesc.Delimiter)
	a.Equal(int64(128), cc.BucketSize)

	// SEQ_CATEGORY_ID(SPARSE(c2, 10000, COMMA), 128)
	scc, ok := trainStmt.Features["feature_columns"][9].(*SeqCategoryIDColumn)
	a.True(ok)
	a.True(scc.FieldDesc.IsSparse)
	a.Equal("c2", scc.FieldDesc.Name)
	a.Equal(10000, scc.FieldDesc.Shape[0])

	// EMBEDDING(c1, 128)
	emb, ok = trainStmt.Features["feature_columns"][10].(*EmbeddingColumn)
	a.True(ok)
	a.Equal(nil, emb.CategoryColumn)
	a.Equal(128, emb.Dimension)

	// EMBEDDING(SPARSE(c2, 10000, COMMA, "int"), 128)
	emb, ok = trainStmt.Features["feature_columns"][11].(*EmbeddingColumn)
	a.True(ok)
	catCol, ok := emb.CategoryColumn.(*CategoryIDColumn)
	a.True(ok)
	a.True(catCol.FieldDesc.IsSparse)
	a.Equal("c2", catCol.FieldDesc.Name)
	a.Equal(10000, catCol.FieldDesc.Shape[0])
	a.Equal(",", catCol.FieldDesc.Delimiter)

	// INDICATOR(CATEGORY_ID(c3, 512)),
	ic, ok := trainStmt.Features["feature_columns"][12].(*IndicatorColumn)
	a.True(ok)
	catCol, ok = ic.CategoryColumn.(*CategoryIDColumn)
	a.True(ok)
	a.Equal("c3", catCol.FieldDesc.Name)
	a.Equal(int64(512), catCol.BucketSize)

	// INDICATOR(c1)
	ic, ok = trainStmt.Features["feature_columns"][13].(*IndicatorColumn)
	a.True(ok)
	a.Equal(nil, ic.CategoryColumn)
	a.Equal("c1", ic.Name)

	// INDICATOR(SPARSE(c2, 10000, COMMA, "int"))
	ic, ok = trainStmt.Features["feature_columns"][14].(*IndicatorColumn)
	a.True(ok)
	catCol, ok = ic.CategoryColumn.(*CategoryIDColumn)
	a.True(ok)
	a.True(catCol.FieldDesc.IsSparse)
	a.Equal("c2", catCol.FieldDesc.Name)
	a.Equal(10000, catCol.FieldDesc.Shape[0])

	// WEIGHTED_CATEGORY(CATEGORY_ID(SPARSE(c2, 10000, "-", "int", ":", "string"), 128)) verify
	// sparse with two delimiters (k:v-k:v-k:v)
	wcc, ok := trainStmt.Features["feature_columns"][15].(*WeightedCategoryColumn)
	a.True(ok)
	cc, ok = wcc.CategoryColumn.(*CategoryIDColumn)
	a.True(ok)
	a.True(cc.FieldDesc.IsSparse)
	a.Equal("-", cc.FieldDesc.Delimiter)
	a.Equal(":", cc.FieldDesc.DelimiterKV)
	a.Equal(Float, cc.FieldDesc.DTypeWeight)

	// EMBEDDING(SPARSE(c2, 10000, "-", "int", ":", "float"), 128, sum)
	emb, ok = trainStmt.Features["feature_columns"][16].(*EmbeddingColumn)
	a.True(ok)
	wcc, ok = emb.CategoryColumn.(*WeightedCategoryColumn)
	a.True(ok)
	cc, ok = wcc.CategoryColumn.(*CategoryIDColumn)
	a.True(ok)
	a.True(cc.FieldDesc.IsSparse)
	a.Equal("-", cc.FieldDesc.Delimiter)

	l, ok := trainStmt.Label.(*NumericColumn)
	a.True(ok)
	a.Equal("c4", l.FieldDesc.Name)

	a.Equal("mymodel", trainStmt.Into)
}

func TestInferStringValue(t *testing.T) {
	a := assert.New(t)
	for _, s := range []string{"true", "TRUE", "True"} {
		a.Equal(inferStringValue(s), true)
		a.Equal(inferStringValue(fmt.Sprintf("\"%s\"", s)), s)
		a.Equal(inferStringValue(fmt.Sprintf("'%s'", s)), s)
	}
	for _, s := range []string{"false", "FALSE", "False"} {
		a.Equal(inferStringValue(s), false)
		a.Equal(inferStringValue(fmt.Sprintf("\"%s\"", s)), s)
		a.Equal(inferStringValue(fmt.Sprintf("'%s'", s)), s)
	}
	a.Equal(inferStringValue("t"), "t")
	a.Equal(inferStringValue("F"), "F")
	a.Equal(inferStringValue("1"), 1)
	a.Equal(inferStringValue("\"1\""), "1")
	a.Equal(inferStringValue("'1'"), "1")
	a.Equal(inferStringValue("2.3"), float32(2.3))
	a.Equal(inferStringValue("\"2.3\""), "2.3")
	a.Equal(inferStringValue("'2.3'"), "2.3")
}

func bucketColumnParserTestMain(bucketStr string) error {
	stmtStr := fmt.Sprintf(`
	SELECT petal_length, class
	FROM iris.train
	TO TRAIN sqlflow_models.my_bucket_column_model
	WITH model.batch_size = 32
	COLUMN BUCKET(%s)
	LABEL class
	INTO db.explain_result;
	`, bucketStr)

	pr, err := parser.Parse("mysql", stmtStr)

	if err != nil {
		return err
	}

	trainStmt, err := GenerateTrainStmt(pr[0].SQLFlowSelectStmt)
	if err != nil {
		return err
	}

	if _, ok := trainStmt.Features["feature_columns"][0].(*BucketColumn); !ok {
		return fmt.Errorf("feature column should be BucketColumn")
	}

	return nil
}

func TestBucketColumnParser(t *testing.T) {
	a := assert.New(t)
	a.NoError(bucketColumnParserTestMain("DENSE(petal_length, 1), [0, 10]"))
	a.NoError(bucketColumnParserTestMain("DENSE(petal_length, 1), [-10, -5, 10]"))
	a.NoError(bucketColumnParserTestMain("petal_length, [10, 20]"))
	a.NoError(bucketColumnParserTestMain("petal_length, [-100]"))
	a.NoError(bucketColumnParserTestMain("petal_length, [-100, -50]"))

	a.Error(bucketColumnParserTestMain("DENSE(petal_length, 1), [10, 0]"))
	a.Error(bucketColumnParserTestMain("DENSE(petal_length, 1), [-10, -10]"))
	a.Error(bucketColumnParserTestMain("DENSE(petal_length, 1), [5, 5]"))
}

func TestGenerateTrainStmtModelZoo(t *testing.T) {
	a := assert.New(t)

	normal := `
	SELECT c1, c2, c3, c4
	FROM my_table
	TO TRAIN a_data_scientist/regressors:v0.2/MyDNNRegressor
	WITH
		model.n_classes=2,
		train.optimizer="adam"
	LABEL c4
	INTO mymodel;
	`

	r, e := parser.ParseStatement("mysql", normal)
	a.NoError(e)

	trainStmt, err := GenerateTrainStmt(r.SQLFlowSelectStmt)
	a.NoError(err)
	a.Equal("a_data_scientist/regressors:v0.2", trainStmt.ModelImage)
	a.Equal("MyDNNRegressor", trainStmt.Estimator)
}

func TestGenerateRunStmt(t *testing.T) {
	a := assert.New(t)

	{
		testToRun := `
SELECT * FROM source_table
TO RUN a_data_scientist/ts_data_processor:1.0;`

		r, e := parser.ParseStatement("mysql", testToRun)
		a.NoError(e)

		runStmt, e := GenerateRunStmt(r.SQLFlowSelectStmt)
		a.NoError(e)

		a.True(runStmt.IsExtended())
		a.Equal(`SELECT * FROM source_table`, runStmt.Select)
		a.Equal(`a_data_scientist/ts_data_processor:1.0`, runStmt.ImageName)
		a.Equal(0, len(runStmt.Parameters))
		a.Equal(0, len(runStmt.Into))
	}

	{
		testToRun := `
SELECT * FROM source_table
TO RUN a_data_scientist/ts_data_processor:1.0
CMD "slide_window_to_row";`

		r, e := parser.ParseStatement("mysql", testToRun)
		a.NoError(e)

		runStmt, e := GenerateRunStmt(r.SQLFlowSelectStmt)
		a.NoError(e)

		a.True(runStmt.IsExtended())
		a.Equal(`SELECT * FROM source_table`, runStmt.Select)
		a.Equal(`a_data_scientist/ts_data_processor:1.0`, runStmt.ImageName)
		a.True(reflect.DeepEqual(runStmt.Parameters, []string{`slide_window_to_row`}))
		a.Equal(0, len(runStmt.Into))
	}

	{
		testToRun := `
SELECT * FROM source_table 
TO RUN a_data_scientist/ts_data_processor:1.0
CMD "slide_window_to_row"
INTO output_table;`

		r, e := parser.ParseStatement("mysql", testToRun)
		a.NoError(e)

		runStmt, e := GenerateRunStmt(r.SQLFlowSelectStmt)
		a.NoError(e)

		a.True(runStmt.IsExtended())
		a.Equal(`SELECT * FROM source_table`, runStmt.Select)
		a.Equal(`a_data_scientist/ts_data_processor:1.0`, runStmt.ImageName)
		a.True(reflect.DeepEqual(runStmt.Parameters, []string{`slide_window_to_row`}))
		a.Equal(`output_table`, runStmt.Into)
	}

	{
		testToRun := `
SELECT * FROM source_table 
TO RUN a_data_scientist/ts_data_processor:1.0
CMD "slide_window_to_row"
INTO output_table_1, output_table_2;`

		r, e := parser.ParseStatement("mysql", testToRun)
		a.NoError(e)

		runStmt, e := GenerateRunStmt(r.SQLFlowSelectStmt)
		a.NoError(e)

		a.True(runStmt.IsExtended())
		a.Equal(`SELECT * FROM source_table`, runStmt.Select)
		a.Equal(`a_data_scientist/ts_data_processor:1.0`, runStmt.ImageName)
		a.True(reflect.DeepEqual(runStmt.Parameters, []string{`slide_window_to_row`}))
		a.Equal(`output_table_1,output_table_2`, runStmt.Into)
	}

	{
		testToRun := `
SELECT * FROM source_table 
TO RUN a_data_scientist/ts_data_processor:1.0
CMD "slide_window_to_row", "--param_a=value_a", "--param_b=value_b"
INTO output_table_1, output_table_2;`

		r, e := parser.ParseStatement("mysql", testToRun)
		a.NoError(e)

		runStmt, e := GenerateRunStmt(r.SQLFlowSelectStmt)
		a.NoError(e)

		a.True(runStmt.IsExtended())
		a.Equal(`SELECT * FROM source_table`, runStmt.Select)
		a.Equal(`a_data_scientist/ts_data_processor:1.0`, runStmt.ImageName)
		a.True(reflect.DeepEqual(
			runStmt.Parameters,
			[]string{
				`slide_window_to_row`,
				`--param_a=value_a`,
				`--param_b=value_b`,
			}))
		a.Equal(`output_table_1,output_table_2`, runStmt.Into)
	}
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
	a.NoError(model.MockInDB(cwd, `SELECT * FROM iris.train
TO TRAIN DNNClassifier
WITH model.n_classes=3, model.hidden_units=[10,20]
LABEL class
INTO sqlflow_models.mymodel;`, "sqlflow_models.mymodel"))

	predStmt, err := GeneratePredictStmt(r.SQLFlowSelectStmt, database.GetTestingDBSingleton().URL(), "", cwd, true)
	a.NoError(err)

	a.Equal("iris.predict", predStmt.ResultTable)
	a.Equal("class", predStmt.TrainStmt.Label.GetFieldDesc()[0].Name)
	a.Equal("DNNClassifier", predStmt.TrainStmt.Estimator)
	nc, ok := predStmt.TrainStmt.Features["feature_columns"][0].(*NumericColumn)
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
	a.NoError(model.MockInDB(cwd, `SELECT * FROM iris.train
TO TRAIN xgboost.gbtree
WITH
  objective="multi:softprob",
  train.num_boost_round = 30,
  eta = 0.4,
  num_class = 3
LABEL class
INTO sqlflow_models.my_xgboost_model;`, "sqlflow_models.my_xgboost_model"))

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

	ExplainStmt, e := GenerateExplainStmt(pr.SQLFlowSelectStmt, connStr, modelDir, cwd, true)
	a.NoError(e)
	a.Equal(ExplainStmt.Explainer, "TreeExplainer")
	a.Equal(len(ExplainStmt.Attributes), 3)
	a.Equal(ExplainStmt.Attributes["summary.sort"], true)
	a.Equal(ExplainStmt.Attributes["summary.plot_type"], "bar")
	a.Equal(ExplainStmt.Attributes["summary.alpha"], 1)

	nc, ok := ExplainStmt.TrainStmt.Features["feature_columns"][0].(*NumericColumn)
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

	ExplainIntoStmt, e := GenerateExplainStmt(pr.SQLFlowSelectStmt, connStr, modelDir, cwd, true)
	a.NoError(e)
	a.Equal(ExplainIntoStmt.Explainer, "TreeExplainer")
	a.Equal(len(ExplainIntoStmt.Attributes), 3)
	a.Equal("db.explain_result", ExplainIntoStmt.Into)

	pr, e = parser.ParseStatement("mysql", `SELECT * FROM iris.train TO EXPLAIN sqlflow_models.my_xgboost_model;`)
	a.NoError(e)
	shortExplainStmt, e := GenerateExplainStmt(pr.SQLFlowSelectStmt, connStr, modelDir, cwd, true)
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
	stmt, e := GenerateOptimizeStmt(r[0].SQLFlowSelectStmt)
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
	stmt, e = GenerateOptimizeStmt(r[0].SQLFlowSelectStmt)
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
	stmt, e = GenerateOptimizeStmt(r[0].SQLFlowSelectStmt)
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
