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

package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/go/database"
	pb "sqlflow.org/sqlflow/go/proto"
	server "sqlflow.org/sqlflow/go/sqlflowserver"
)

func caseCustomLoopModel(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
TO TRAIN sqlflow_models.CustomClassifier
LABEL class
INTO sqlflow_models.custom_loop_model;`, caseTrainTable)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}
	predSQL := fmt.Sprintf(`SELECT * FROM %s
TO PREDICT sqlflow_models.custom_loop_model_pred_result.class
USING sqlflow_models.custom_loop_model;`, caseTrainTable)
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}
	evalSQL := fmt.Sprintf(`SELECT * FROM %s
TO EVALUATE sqlflow_models.custom_loop_model
WITH validation.metrics="Accuracy"
LABEL class
INTO sqlflow_models.custom_loop_model_eval_result;`, caseTrainTable)
	_, _, _, err = connectAndRunSQL(evalSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}
}

func caseTrainXGBoostWithNull(t *testing.T) {
	a := assert.New(t)
	prepareSQL1 := `CREATE TABLE IF NOT EXISTS boston.train_ext AS SELECT * FROM boston.train;`
	prepareSQL2 := `UPDATE boston.train_ext
SET rad = NULL
WHERE zn < 18.1 AND zn > 17.0;`
	prepareSQL3 := `UPDATE boston.train_ext
SET tax = NULL
WHERE zn < 18.1 AND zn > 17.0;`
	_, _, _, err := connectAndRunSQL(prepareSQL1)
	a.NoError(err)
	_, _, _, err = connectAndRunSQL(prepareSQL2)
	a.NoError(err)
	_, _, _, err = connectAndRunSQL(prepareSQL3)
	a.NoError(err)

	trainSQL := fmt.Sprintf(`SELECT * FROM boston.train_ext
TO TRAIN xgboost.gbtree
WITH
	objective="reg:squarederror",
	train.num_boost_round = 30
LABEL medv
INTO sqlflow_models.my_xgb_regression_model;
`)
	_, _, _, err = connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func caseTrainXGBoostMultiClass(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`
SELECT * FROM iris.train
TO TRAIN xgboost.gbtree
WITH
	objective="multi:softmax",
	num_class=3
LABEL class
INTO sqlflow_models.my_xgb_multi_model;
`)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}

	predSQL := `
SELECT * FROM iris.test
TO PREDICT iris.xgb_pred_result.class
USING sqlflow_models.my_xgb_multi_model;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run predSQL error: %v", err)
	}

	showPred := `SELECT * FROM iris.xgb_pred_result LIMIT 5;`
	_, rows, _, err := connectAndRunSQL(showPred)
	if err != nil {
		a.Fail("Run showPred error: %v", err)
	}
	for _, row := range rows {
		a.True(EqualAny(int64(0), row[4]) || EqualAny(int64(1), row[4]) || EqualAny(int64(2), row[4]))
	}

	trainSQL = fmt.Sprintf(`
SELECT * FROM iris.train WHERE class < 2
TO TRAIN xgboost.gbtree
WITH
	objective="binary:logistic"
LABEL class
INTO sqlflow_models.my_xgb_multi_model;
`)
	_, _, _, err = connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}

	predSQL = `
SELECT * FROM iris.test WHERE class < 2
TO PREDICT iris.xgb_pred_result.class
USING sqlflow_models.my_xgb_multi_model;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run predSQL error: %v", err)
	}
	showPred = `SELECT * FROM iris.xgb_pred_result LIMIT 5;`
	_, rows, _, err = connectAndRunSQL(showPred)
	if err != nil {
		a.Fail("Run showPred error: %v", err)
	}
	for _, row := range rows {
		a.True(EqualAny(int64(0), row[4]) || EqualAny(int64(1), row[4]))
	}
}

func TestEnd2EndMySQL(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST_DB") != "mysql" {
		t.Skip("Skipping mysql tests")
	}
	dbConnStr = database.GetTestingMySQLURL()
	modelDir := ""

	tmpDir, caCrt, caKey, err := generateTempCA()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatalf("failed to generate CA pair %v", err)
	}
	go start(modelDir, caCrt, caKey, unitTestPort, false)
	server.WaitPortReady(fmt.Sprintf("localhost:%d", unitTestPort), 0)
	err = prepareTestData(dbConnStr)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}

	t.Run("CaseShowDatabases", caseShowDatabases)
	t.Run("CaseSelect", caseSelect)
	t.Run("CaseInsert", caseInsert)
	t.Run("CaseShouldError", CaseShouldError)
	t.Run("CaseTrainSQL", caseTrainSQL)
	t.Run("caseCoverageCommon", caseCoverageCommon)
	t.Run("caseCoverageCustomModel", caseCoverageCustomModel)
	t.Run("CaseCoverage", CaseCoverageMysql)
	t.Run("CaseTrainWithCommaSeparatedLabel", CaseTrainWithCommaSeparatedLabel)
	t.Run("CaseTrainCustomModelFunctional", CaseTrainCustomModelFunctional)
	t.Run("CaseCustomLoopModel", caseCustomLoopModel)
	t.Run("CaseSQLByPassLeftJoin", CaseSQLByPassLeftJoin)
	t.Run("CaseTrainRegression", caseTrainRegression)
	t.Run("CaseScoreCard", caseScoreCard)

	// Cases using feature derivation
	t.Run("CaseFeatureDerivation", CaseFeatureDerivation)

	t.Run("caseWeightedKeyValueColumn", caseWeightedKeyValueColumn)

	// xgboost cases
	t.Run("caseTrainXGBoostWithNull", caseTrainXGBoostWithNull)
	t.Run("caseTrainXGBoostMultiClass", caseTrainXGBoostMultiClass)
	t.Run("caseTrainXGBoostRegressionConvergence", caseTrainXGBoostRegressionConvergence)
	t.Run("CasePredictXGBoostRegression", casePredictXGBoostRegression)

	caseTensorFlowIncrementalTrain(t, false)

	caseXGBoostFeatureColumn(t, false)

	t.Run("CaseShowTrain", caseShowTrain)

	// Cases for diagnosis
	t.Run("CaseDiagnosisMissingModelParams", CaseDiagnosisMissingModelParams)

	t.Run("CaseTrainARIMAWithSTLDecompositionModel", caseTrainARIMAWithSTLDecompositionModel)

	t.Run("CaseEnd2EndCrossFeatureColumn", caseEnd2EndCrossFeatureColumn)

	t.Run("CaseXGBoostSparseKeyValueColumn", caseXGBoostSparseKeyValueColumn)
	t.Run("CaseEnd2EndXGBoostDenseFeatureColumn", func(t *testing.T) {
		caseEnd2EndXGBoostDenseFeatureColumn(t, false)
	})

	// Cases for optimize
	t.Run("CaseTestOptimizeClauseWithoutGroupBy", caseTestOptimizeClauseWithoutGroupBy)
	t.Run("CaseTestOptimizeClauseWithGroupBy", caseTestOptimizeClauseWithGroupBy)
	t.Run("CaseTestOptimizeClauseWithBinaryVarType", caseTestOptimizeClauseWithBinaryVarType)
	t.Run("CaseTestOptimizeClauseWithoutConstraint", caseTestOptimizeClauseWithoutConstraint)
}

func caseInsert(t *testing.T) {
	const sqlTmpl = `DROP TABLE IF EXISTS %[1]s;
	CREATE TABLE %[1]s (c BIGINT);
	INSERT INTO %[1]s VALUES('1');`

	table := caseDB + ".non_exist_table_name"
	dropTableSQL := fmt.Sprintf(`DROP TABLE %s;`, table)
	defer connectAndRunSQL(dropTableSQL)

	a := assert.New(t)
	_, _, _, err := connectAndRunSQL(fmt.Sprintf(sqlTmpl, table))
	a.NoError(err)

	countSQL := fmt.Sprintf(`SELECT * FROM %s;`, table)
	_, rows, _, err := connectAndRunSQL(countSQL)
	a.NoError(err)
	a.Equal(1, len(rows))
}

func caseScoreCard(t *testing.T) {
	a := assert.New(t)
	sql := `SELECT * FROM scorecard.train
TO TRAIN sqlflow_models.ScoreCard
LABEL serious_dlqin2yrs 
INTO sqlflow_models.my_scorecard_model;`
	_, _, _, err := connectAndRunSQL(sql)
	a.NoError(err)
}

func CaseShouldError(t *testing.T) {
	cases := []string{`SELECT * FROM iris.train LIMIT 0 TO TRAIN xgboost.gbtree
WITH objective="reg:squarederror"
LABEL class 
INTO sqlflow_models.my_xgb_regression_model;`, // empty dataset
		`SELECT * FROM iris.train WHERE class=2 TO TRAIN xgboost.gbtree
WITH objective="reg:squarederror"
LABEL target
INTO sqlflow_models.my_xgb_regression_model;`, // label not exist
		`SELECT * FROM housing.train
TO TRAIN DNNRegressora WITH
    model.hidden_units = [10, 20],
    validation.select = "SELECT * FROM %s "
COLUMN INDICATOR(CATEGORY_ID("a.*", 1000))
LABEL target
INTO housing.dnn_model;`, // column regex don't match any column
		`SELECT * FROM housing.train
TO TRAIN DNNRegressor WITH
    model.hidden_units = [10, 20],
    validation.select = "SELECT * FROM %s "
COLUMN INDICATOR(CATEGORY_ID("[*", 1000))
LABEL target
INTO housing.dnn_model;`, // invalid column regex
		`SELECT * FROM iris.train
TO TRAIN DNNClassifier WITH
	model.n_classes = 3,
	model.hidden_units = [10, 20],
	validation.select = "SELECT * FROM %s LIMIT 30"
COLUMN typo, sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO iris.dnn_model;`, // typo in column clause
	}
	for _, sql := range cases {
		connectAndRunSQLShouldError(sql)
	}
}

func CaseFeatureDerivation(t *testing.T) {
	cases := []string{`SELECT * FROM iris.train
TO TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20]
LABEL class
INTO sqlflow_models.my_dnn_model;`, // basic case, derive numeric
		`SELECT * FROM iris.test
TO PREDICT iris.predict.class
USING sqlflow_models.my_dnn_model;`, // predict using feature derivation model
		`SELECT c1, c2, c3, c4, c5, class from feature_derivation_case.train
TO TRAIN DNNClassifier
WITH model.n_classes=3, model.hidden_units=[10,10]
COLUMN EMBEDDING(c3, 32, sum), EMBEDDING(SPARSE(c5, 64, COMMA), 32)
LABEL class
INTO sqlflow_models.my_dnn_model;`, // general case to derive all column types
		`SELECT c1, c2, c3, c4, c5, class from feature_derivation_case.train
TO TRAIN DNNClassifier
WITH model.n_classes=3, model.hidden_units=[10,10]
COLUMN INDICATOR(c3), EMBEDDING(SPARSE(c5, 64, COMMA), 32, sum)
LABEL class
INTO sqlflow_models.my_dnn_model;`, // general case with indicator column
		`SELECT news_title, class_id
FROM text_cn.train_processed
TO TRAIN DNNClassifier
WITH model.n_classes = 17, model.hidden_units = [10, 20]
COLUMN EMBEDDING(CATEGORY_ID(SPARSE(news_title,16000,COMMA), 16000),128,mean)
LABEL class_id
INTO sqlflow_models.my_dnn_model;`, // specify COLUMN
		`SELECT news_title, class_id
FROM text_cn.train_processed
TO TRAIN DNNClassifier
WITH model.n_classes = 17, model.hidden_units = [10, 20]
COLUMN EMBEDDING(SPARSE(news_title,16000,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_dnn_model;`, // derive CATEGORY_ID()
		`SELECT * FROM housing.train
TO TRAIN xgboost.gbtree
WITH objective="reg:squarederror",
	 train.num_boost_round=30
LABEL target
INTO sqlflow_models.my_xgb_regression_model;`, // xgboost feature derivation
		`SELECT * FROM housing.test
TO PREDICT housing.predict.target
USING sqlflow_models.my_xgb_regression_model;`, // predict xgboost feature derivation model
	}

	a := assert.New(t)
	for _, sql := range cases {
		_, _, _, err := connectAndRunSQL(sql)
		a.NoError(err)
	}
}

func CaseDiagnosisMissingModelParams(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT * FROM iris.train TO TRAIN DNNClassifier WITH
  model.n_classes = 3,
  train.epoch = 10
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	a.Contains(err.Error(), "DNNClassifierV2 missing 1 required attribute: 'hidden_units'")
}

func CaseTrainCustomModelFunctional(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
TO TRAIN sqlflow_models.dnnclassifier_functional_model
WITH model.n_classes = 3, validation.metrics="CategoricalAccuracy"
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO %s;`, caseTrainTable, caseInto)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func CaseTrainWithCommaSeparatedLabel(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT sepal_length, sepal_width, petal_length, concat(petal_width,',',class) as class FROM iris.train 
TO TRAIN sqlflow_models.RNNBasedTimeSeriesModel WITH
	model.n_in=3,
	model.stack_units = [10, 10],
	model.n_out=2,
	model.model_type="lstm",
	validation.metrics= "MeanAbsoluteError,MeanSquaredError"
LABEL class
INTO sqlflow_models.my_dnn_regts_model_2;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}

	predSQL := `SELECT sepal_length, sepal_width, petal_length, concat(petal_width,',',class) as class FROM iris.test 
TO PREDICT iris.predict_ts_2.class USING sqlflow_models.my_dnn_regts_model_2;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}

	showPred := `SELECT * FROM iris.predict_ts_2 LIMIT 5;`
	_, rows, _, err := connectAndRunSQL(showPred)
	if err != nil {
		a.Fail("Run showPred error: %v", err)
	}

	for _, row := range rows {
		// NOTE: Ensure that the predict result contains comma
		AssertIsSubStringAny(a, ",", row[3])
	}
}

func CaseSQLByPassLeftJoin(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT f1.user_id, f1.fea1, f2.fea2
FROM standard_join_test.user_fea1 AS f1 LEFT OUTER JOIN standard_join_test.user_fea2 AS f2
ON f1.user_id = f2.user_id
WHERE f1.user_id < 3;`

	conn, err := createRPCConn()
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, sqlRequest(trainSQL))
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	// wait train finish
	_, _, _, e := ParseResponse(stream)
	a.NoError(e)
}

func CaseXGBoost(t *testing.T) {
	cases := []string{
		`SELECT * FROM iris.train TO TRAIN xgboost.gbtree
WITH objective="multi:softprob",
	 num_class=3,
	 validation.select="SELECT * FROM iris.test",
	 eval_metric=accuracy_score
LABEL class
INTO sqlflow_models.my_xgb_multi_class_model;`, // xgb multi-class training with eval_metric
		`SELECT * FROM iris.test
TO EVALUATE sqlflow_models.my_xgb_multi_class_model
WITH validation.metrics="accuracy_score"
LABEL class
INTO sqlflow_models.my_xgb_regression_model_eval_result;`, // xgb multi-class evaluation
		`SELECT * FROM housing.train TO TRAIN xgboost.gbtree
WITH objective="reg:squarederror",
	 train.num_boost_round = 30,
	 train.batch_size=20
COLUMN f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13
LABEL target
INTO sqlflow_models.my_xgb_regression_model;`, // xgb regression training
		`SELECT * FROM housing.train
TO EXPLAIN sqlflow_models.my_xgb_regression_model
WITH summary.plot_type="bar",
     summary.alpha=1,
     summary.sort=True
USING TreeExplainer;`, // xgb regression explain
		`SELECT * FROM iris.train WHERE class in (0, 1) TO TRAIN xgboost.gbtree
WITH objective="binary:logistic", eval_metric=auc, train.disk_cache=True
LABEL class
INTO sqlflow_models.my_xgb_binary_classification_model;`, // xgb training with external memory
	}

	a := assert.New(t)
	for _, sql := range cases {
		_, _, _, err := connectAndRunSQL(sql)
		a.NoError(err)
	}
}

func CaseCoverageMysql(t *testing.T) {
	cases := []string{
		`SELECT * FROM iris.train WHERE class<>2
TO TRAIN DNNClassifier
WITH
	model.n_classes = 2,
	model.hidden_units = [10, 20],
	validation.select = "SELECT * FROM iris.test WHERE class <>2 LIMIT 30"
LABEL class
INTO sqlflow_models.dnn_binary_classfier;`, // train a binary classification model for evaluation
		`SELECT * FROM iris.test WHERE class<>2
TO EVALUATE sqlflow_models.dnn_binary_classfier
WITH validation.metrics = "Accuracy,AUC"
LABEL class
INTO iris.evaluation_result;`, // evaluate the model
		`SELECT f9, target FROM housing.train
TO TRAIN DNNRegressor WITH model.hidden_units = [10, 20]
COLUMN EMBEDDING(CATEGORY_ID(f9, 1000), 2, "sum")
LABEL target
INTO housing.dnn_model;`, // train a model to predict categorical value
		`SELECT f9, target FROM housing.test
TO PREDICT housing.predict.class USING housing.dnn_model;`, // predict categorical value
		`SELECT f9, f10, target FROM housing.train
TO TRAIN DNNLinearCombinedRegressor WITH
		model.dnn_hidden_units = [10, 20]
COLUMN EMBEDDING(CATEGORY_ID(f9, 25), 2, "sum") for dnn_feature_columns
COLUMN INDICATOR(CATEGORY_ID(f10, 712)) for linear_feature_columns
LABEL target
INTO housing.dnnlinear_model;`, // deep wide model
		`SELECT f9, f10, target FROM housing.test
TO PREDICT housing.predict.class USING housing.dnnlinear_model;`, // deep wide model predict
		`SELECT * FROM housing.train
TO TRAIN DNNRegressor WITH
    model.hidden_units = [10, 20],
    validation.select = "SELECT * FROM housing.test"
COLUMN INDICATOR(CATEGORY_ID("f10|f9|f4", 1000))
LABEL target
INTO housing.dnn_model;`, // column regex
		`SELECT * FROM housing.test
TO PREDICT housing.predict.class
USING housing.dnn_model;`, // column regex mode predict
		`SELECT * FROM iris.train
TO TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20],
	 train.batch_size = 10, train.epoch = 6,
	 train.max_steps = 200,
	 train.save_checkpoints_steps=10,
	 train.log_every_n_iter=20,
	 validation.start_delay_secs=10, validation.throttle_secs=10
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;`, // train with hyper params
		`SELECT news_title, class_id
FROM text_cn.train
TO TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20]
COLUMN EMBEDDING(SPARSE(news_title,16000,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_dnn_model;`, // sparse feature support
		`SELECT news_title, class_id
FROM text_cn.train_processed
TO TRAIN DNNClassifier
WITH model.n_classes = 17, model.hidden_units = [10, 20]
COLUMN EMBEDDING(CATEGORY_ID(news_title,16000,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_dnn_model;`, // dnn text classification
		`SELECT news_title, class_id
FROM text_cn.train_processed
TO TRAIN sqlflow_models.StackedRNNClassifier
WITH model.n_classes = 17, model.stack_units = [16], model.model_type = "lstm", model.bidirectional = True,
	 train.epoch = 1, train.batch_size = 32
COLUMN EMBEDDING(SEQ_CATEGORY_ID(news_title,1600,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_rnn_model;`, // custom rnn model text classification
		`SELECT * FROM iris.train WHERE class!=2
TO TRAIN BoostedTreesClassifier
WITH
	model.n_batches_per_layer=1,
	model.center_bias=True,
	train.batch_size=100,
	train.epoch=10,
	validation.select="SELECT * FROM iris.test where class!=2"
LABEL class
INTO sqlflow_models.boostedtrees_model;`, // train tf boosted trees model
		`SELECT * FROM iris.test WHERE class!=2
TO EXPLAIN sqlflow_models.boostedtrees_model
INTO iris.explain_result;`, // explain tf boosted trees model
	}
	a := assert.New(t)
	for _, sql := range cases {
		_, _, _, err := connectAndRunSQL(sql)
		a.NoError(err)
	}
	// check tf boosted trees model explain result
	getExplainResult := `SELECT * FROM iris.explain_result;`
	_, rows, _, err := connectAndRunSQL(getExplainResult)
	a.NoError(err)
	for _, row := range rows {
		AssertGreaterEqualAny(a, row[1], float32(0))
	}
}

func caseTrainARIMAWithSTLDecompositionModel(t *testing.T) {
	a := assert.New(t)

	trainSQL := `
SELECT time, %[1]s FROM fund.train
TO TRAIN sqlflow_models.ARIMAWithSTLDecomposition
WITH
  model.order=[7, 0, 2],
  model.period=[7, 30],
  model.date_format="%[2]s",
  model.forecast_start='2014-09-01',
  model.forecast_end='2014-09-30'
LABEL %[1]s
INTO fund.%[1]s_model;
`

	var err error

	dateFormat := "%Y-%m-%d"
	purchaseTrainSQL := fmt.Sprintf(trainSQL, "purchase", dateFormat)
	_, _, _, err = connectAndRunSQL(purchaseTrainSQL)
	a.NoError(err)

	redeemTrainSQL := fmt.Sprintf(trainSQL, "redeem", dateFormat)
	_, _, _, err = connectAndRunSQL(redeemTrainSQL)
	a.NoError(err)
}

func caseEnd2EndCrossFeatureColumn(t *testing.T) {
	sqls := []string{`SELECT * FROM iris.train 
TO TRAIN DNNClassifier 
WITH 
	model.n_classes = 3, 
	model.hidden_units=[10, 20] 
COLUMN EMBEDDING(CROSS([petal_width, petal_length], 10), 128, 'sum')
LABEL class 
INTO iris.cross_tf_e2e_test_model;
`,
		`SELECT petal_width, petal_length, class FROM iris.train 
TO TRAIN DNNClassifier 
WITH 
	model.n_classes = 3, 
	model.hidden_units=[10, 20] 
COLUMN EMBEDDING(CROSS([petal_width, petal_length], 10), 128, 'sqrtn')
LABEL class 
INTO iris.cross_tf_e2e_test_model;
`,
	}

	a := assert.New(t)
	for _, sql := range sqls {
		_, _, _, err := connectAndRunSQL(sql)
		a.NoError(err)
	}
}
