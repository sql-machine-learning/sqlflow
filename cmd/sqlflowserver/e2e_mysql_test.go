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
	"sqlflow.org/sqlflow/pkg/database"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/server"
)

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
	t.Run("CaseEmptyDataset", CaseEmptyDataset)
	t.Run("CaseLabelColumnNotExist", CaseLabelColumnNotExist)
	t.Run("CaseTrainSQL", caseTrainSQL)
	t.Run("CaseTrainAndEvaluate", CaseTrainAndEvaluate)
	t.Run("CaseTrainPredictCategoricalFeature", CaseTrainPredictCategoricalFeature)
	t.Run("CaseTrainRegex", CaseTrainRegex)
	t.Run("CaseTypoInColumnClause", CaseTypoInColumnClause)
	t.Run("CaseTrainWithCommaSeparatedLabel", CaseTrainWithCommaSeparatedLabel)

	t.Run("CaseTrainBoostedTreesEstimatorAndExplain", CaseTrainBoostedTreesEstimatorAndExplain)
	t.Run("CaseTrainSQLWithMetrics", caseTrainSQLWithMetrics)
	t.Run("TestTextClassification", CaseTrainTextClassification)
	t.Run("CaseTrainTextClassificationCustomLSTM", CaseTrainTextClassificationCustomLSTM)
	t.Run("CaseTrainCustomModel", caseTrainCustomModel)
	t.Run("CaseTrainCustomModelFunctional", CaseTrainCustomModelFunctional)
	t.Run("CaseTrainOptimizer", caseTrainOptimizer)
	t.Run("CaseTrainSQLWithHyperParams", CaseTrainSQLWithHyperParams)
	t.Run("CaseTrainCustomModelWithHyperParams", CaseTrainCustomModelWithHyperParams)
	t.Run("CaseSparseFeature", CaseSparseFeature)
	t.Run("CaseSQLByPassLeftJoin", CaseSQLByPassLeftJoin)
	t.Run("CaseTrainRegression", caseTrainRegression)
	t.Run("CaseTrainXGBoostRegression", caseTrainXGBoostRegression)
	t.Run("CaseTrainXGBoostMultiClass", CaseTrainXGBoostMultiClass)

	t.Run("CasePredictXGBoostRegression", casePredictXGBoostRegression)
	t.Run("CaseTrainAndExplainXGBoostModel", CaseTrainAndExplainXGBoostModel)

	t.Run("CaseTrainDeepWideModel", caseTrainDeepWideModel)
	t.Run("CaseTrainDeepWideModelOptimizer", caseTrainDeepWideModelOptimizer)
	t.Run("CaseTrainAdaNetAndExplain", caseTrainAdaNetAndExplain)

	// Cases using feature derivation
	t.Run("CaseTrainTextClassificationIR", CaseTrainTextClassificationIR)
	t.Run("CaseTrainTextClassificationFeatureDerivation", CaseTrainTextClassificationFeatureDerivation)
	t.Run("CaseXgboostFeatureDerivation", CaseXgboostFeatureDerivation)
	t.Run("CaseXgboostEvalMetric", CaseXgboostEvalMetric)
	t.Run("CaseXgboostExternalMemory", CaseXgboostExternalMemory)
	t.Run("CaseTrainFeatureDerivation", caseTrainFeatureDerivation)

	t.Run("CaseShowTrain", caseShowTrain)

	// Cases for diagnosis
	t.Run("CaseDiagnosisMissingModelParams", CaseDiagnosisMissingModelParams)

	caseXGBoostFeatureColumn(t, false)
}

func CaseEmptyDataset(t *testing.T) {
	trainSQL := `SELECT * FROM iris.train LIMIT 0 TO TRAIN xgboost.gbtree
WITH objective="reg:squarederror"
LABEL class 
INTO sqlflow_models.my_xgb_regression_model;`
	connectAndRunSQLShouldError(trainSQL)
}

func CaseLabelColumnNotExist(t *testing.T) {
	trainSQL := `SELECT * FROM iris.train WHERE class=2 TO TRAIN xgboost.gbtree
WITH objective="reg:squarederror"
LABEL target
INTO sqlflow_models.my_xgb_regression_model;`
	connectAndRunSQLShouldError(trainSQL)
}

func CaseXgboostFeatureDerivation(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT * FROM housing.train
TO TRAIN xgboost.gbtree
WITH objective="reg:squarederror",
	 train.num_boost_round=30
LABEL target
INTO sqlflow_models.my_xgb_regression_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run test error: %v", err)
	}

	predSQL := `SELECT * FROM housing.test
TO PREDICT housing.predict.target
USING sqlflow_models.my_xgb_regression_model;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run test error: %v", err)
	}
}

func CaseXgboostEvalMetric(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT * FROM iris.train WHERE class in (0, 1) TO TRAIN xgboost.gbtree
WITH objective="binary:logistic", eval_metric=auc
LABEL class
INTO sqlflow_models.my_xgb_binary_classification_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run test error: %v", err)
	}

	predSQL := `SELECT * FROM iris.test TO PREDICT iris.predict.class
USING sqlflow_models.my_xgb_binary_classification_model;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run test error: %v", err)
	}
}

func CaseXgboostExternalMemory(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT * FROM iris.train WHERE class in (0, 1) TO TRAIN xgboost.gbtree
WITH objective="binary:logistic", eval_metric=auc, train.disk_cache=True
LABEL class
INTO sqlflow_models.my_xgb_binary_classification_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run test error: %v", err)
	}

	predSQL := `SELECT * FROM iris.test TO PREDICT iris.predict.class
USING sqlflow_models.my_xgb_binary_classification_model;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run test error: %v", err)
	}
}

func CaseTrainTextClassificationIR(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT news_title, class_id
FROM text_cn.train_processed
TO TRAIN DNNClassifier
WITH model.n_classes = 17, model.hidden_units = [10, 20]
COLUMN EMBEDDING(CATEGORY_ID(SPARSE(news_title,16000,COMMA), 16000),128,mean)
LABEL class_id
INTO sqlflow_models.my_dnn_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
}

func CaseTrainTextClassificationFeatureDerivation(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT news_title, class_id
FROM text_cn.train_processed
TO TRAIN DNNClassifier
WITH model.n_classes = 17, model.hidden_units = [10, 20]
COLUMN EMBEDDING(SPARSE(news_title,16000,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_dnn_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
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
	TO TRAIN sqlflow_models.LSTMBasedTimeSeriesModel WITH
	  model.n_in=3,
	  model.stack_units = [10, 10],
	  model.n_out=2,
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

func CaseTrainTextClassificationCustomLSTM(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT news_title, class_id
FROM text_cn.train_processed
TO TRAIN sqlflow_models.StackedBiLSTMClassifier
WITH model.n_classes = 17, model.stack_units = [16], train.epoch = 1, train.batch_size = 32
COLUMN EMBEDDING(SEQ_CATEGORY_ID(news_title,1600,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_bilstm_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func CaseTrainSQLWithHyperParams(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM iris.train
TO TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20],
	 train.batch_size = 10, train.epoch = 6,
	 train.max_steps = 200,
	 train.save_checkpoints_steps=10,
	 train.log_every_n_iter=20,
	 validation.start_delay_secs=10, validation.throttle_secs=10
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func CaseTrainCustomModelWithHyperParams(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM iris.train
TO TRAIN sqlflow_models.DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20], train.batch_size = 10, train.epoch=2
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model_custom;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func CaseSparseFeature(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT news_title, class_id
FROM text_cn.train
TO TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20]
COLUMN EMBEDDING(SPARSE(news_title,16000,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_dnn_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
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

// CaseTrainXGBoostMultiClass is used to test xgboost regression models
func CaseTrainXGBoostMultiClass(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`
SELECT *
FROM iris.train
TO TRAIN xgboost.gbtree
WITH
	objective="multi:softprob",
	num_class=3,
	validation.select="SELECT * FROM iris.test"
LABEL class
INTO sqlflow_models.my_xgb_multi_class_model;
`)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}

	evalSQL := fmt.Sprintf(`
SELECT * FROM iris.test
TO EVALUATE sqlflow_models.my_xgb_multi_class_model
WITH validation.metrics="accuracy_score"
LABEL class
INTO sqlflow_models.my_xgb_regression_model_eval_result;
`)
	_, _, _, err = connectAndRunSQL(evalSQL)
	if err != nil {
		a.Fail("run evalSQL error: %v", err)
	}
}

// CaseTrainAndExplainXGBoostModel is used to test training a xgboost model,
// then explain it
func CaseTrainAndExplainXGBoostModel(t *testing.T) {
	a := assert.New(t)
	trainStmt := `
SELECT *
FROM housing.train
TO TRAIN xgboost.gbtree
WITH
	objective="reg:squarederror",
	train.num_boost_round = 30,
	train.batch_size=20
COLUMN f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13
LABEL target
INTO sqlflow_models.my_xgb_regression_model;
	`
	explainStmt := `
SELECT *
FROM housing.train
TO EXPLAIN sqlflow_models.my_xgb_regression_model
WITH
    summary.plot_type="bar",
    summary.alpha=1,
    summary.sort=True
USING TreeExplainer;
	`
	conn, err := createRPCConn()
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, sqlRequest(trainStmt))
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	_, _, _, e := ParseResponse(stream)
	a.NoError(e)
	stream, err = cli.Run(ctx, sqlRequest(explainStmt))
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	_, _, _, e = ParseResponse(stream)
	a.NoError(e)
}

func CaseTrainRegex(t *testing.T) {
	a := assert.New(t)
	trainSQL := `
SELECT * FROM housing.train
TO TRAIN DNNRegressor WITH
    model.hidden_units = [10, 20],
    validation.select = "SELECT * FROM housing.test"
COLUMN INDICATOR(CATEGORY_ID("f10|f9|f4", 1000))
LABEL target
INTO housing.dnn_model;
`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := `
	SELECT * FROM housing.test
	TO PREDICT housing.predict.class
	USING housing.dnn_model;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}
	trainSQL = `
SELECT * FROM housing.train
TO TRAIN DNNRegressora WITH
    model.hidden_units = [10, 20],
    validation.select = "SELECT * FROM %s "
COLUMN INDICATOR(CATEGORY_ID("a.*", 1000))
LABEL target
INTO housing.dnn_model;
` // don't match any column
	connectAndRunSQLShouldError(trainSQL)

	trainSQL = `
SELECT * FROM housing.train
TO TRAIN DNNRegressor WITH
    model.hidden_units = [10, 20],
    validation.select = "SELECT * FROM %s "
COLUMN INDICATOR(CATEGORY_ID("[*", 1000))
LABEL target
INTO housing.dnn_model;
` // invalid regex
	connectAndRunSQLShouldError(trainSQL)

}

func CaseTypoInColumnClause(t *testing.T) {
	trainSQL := fmt.Sprintf(`
	SELECT * FROM %s
	TO TRAIN DNNClassifier WITH
		model.n_classes = 3,
		model.hidden_units = [10, 20],
		validation.select = "SELECT * FROM %s LIMIT 30"
	COLUMN typo, sepal_length, sepal_width, petal_length, petal_width
	LABEL class
	INTO %s;
	`, caseTrainTable, caseTrainTable, caseInto)
	connectAndRunSQLShouldError(trainSQL)
}

func CaseTrainBoostedTreesEstimatorAndExplain(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`
	SELECT * FROM iris.train WHERE class!=2
	TO TRAIN BoostedTreesClassifier
	WITH
		model.n_batches_per_layer=1,
		model.center_bias=True,
		train.batch_size=100,
		train.epoch=10,
		validation.select="SELECT * FROM iris.test where class!=2"
	LABEL class
	INTO %s;
	`, caseInto)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	explainSQL := fmt.Sprintf(`SELECT * FROM iris.test WHERE class!=2
	TO EXPLAIN %s
	INTO iris.explain_result;`, caseInto)
	_, _, _, err = connectAndRunSQL(explainSQL)
	a.NoError(err)

	getExplainResult := `SELECT * FROM iris.explain_result;`
	_, rows, _, err := connectAndRunSQL(getExplainResult)
	a.NoError(err)
	for _, row := range rows {
		AssertGreaterEqualAny(a, row[1], float32(0))
	}
}

func CaseTrainTextClassification(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT news_title, class_id
FROM text_cn.train_processed
TO TRAIN DNNClassifier
WITH model.n_classes = 17, model.hidden_units = [10, 20]
COLUMN EMBEDDING(CATEGORY_ID(news_title,16000,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_dnn_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func CaseTrainAndEvaluate(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s WHERE class<>2
TO TRAIN DNNClassifier
WITH
	model.n_classes = 2,
	model.hidden_units = [10, 20],
	validation.select = "SELECT * FROM %s WHERE class <>2 LIMIT 30"
LABEL class
INTO %s;`, caseTrainTable, caseTrainTable, caseInto)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	evalSQL := fmt.Sprintf(`SELECT * FROM %s WHERE class<>2
TO EVALUATE %s
WITH validation.metrics = "Accuracy,AUC"
LABEL class
INTO %s.evaluation_result;`, caseTestTable, caseInto, caseDB)
	_, _, _, err = connectAndRunSQL(evalSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}
}

func CaseTrainPredictCategoricalFeature(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT f9, target FROM housing.train
TO TRAIN DNNRegressor WITH
		model.hidden_units = [10, 20]
COLUMN EMBEDDING(CATEGORY_ID(f9, 1000), 2, "sum")
LABEL target
INTO housing.dnn_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := `SELECT f9, target FROM housing.test
TO PREDICT housing.predict.class USING housing.dnn_model;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}

	trainSQL = `SELECT f9, f10, target FROM housing.train
TO TRAIN DNNLinearCombinedRegressor WITH
		model.dnn_hidden_units = [10, 20]
COLUMN EMBEDDING(CATEGORY_ID(f9, 25), 2, "sum") for dnn_feature_columns
COLUMN INDICATOR(CATEGORY_ID(f10, 712)) for linear_feature_columns
LABEL target
INTO housing.dnnlinear_model;`
	_, _, _, err = connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL = `SELECT f9, f10, target FROM housing.test
TO PREDICT housing.predict.class USING housing.dnnlinear_model;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}
}
