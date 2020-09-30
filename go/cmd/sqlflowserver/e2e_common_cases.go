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
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/golang/protobuf/ptypes/any"
	"sqlflow.org/sqlflow/go/proto"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/go/database"
)

func caseShowDatabases(t *testing.T) {
	a := assert.New(t)
	cmd := "show databases;"
	head, resp, _, err := connectAndRunSQL(cmd)
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	if os.Getenv("SQLFLOW_TEST_DB") == "hive" {
		a.Equal("database_name", head[0])
	} else {
		a.Equal("Database", head[0])
	}

	expectedDBs := map[string]string{
		"information_schema":          "",
		"boston":                      "",
		"churn":                       "",
		"creditcard":                  "",
		"energy":                      "", // energy tutorial table
		"feature_derivation_case":     "",
		"fund":                        "",
		"housing":                     "",
		"iris":                        "",
		"mysql":                       "",
		"optimize_test_db":            "",
		"performance_schema":          "",
		"sqlflow_models":              "",
		"sf_home":                     "", // default auto train&val database
		"sqlfs_test":                  "",
		"sys":                         "",
		"text_cn":                     "",
		"standard_join_test":          "",
		"sanity_check":                "",
		"iris_e2e":                    "", // created by Python e2e test
		"hive":                        "", // if current mysql is also used for hive
		"default":                     "", // if fetching default hive databases
		"sqlflow":                     "", // to save model zoo trained models
		"imdb":                        "",
		"sqlflow_model_zoo":           "",
		"sqlflow_public_models":       "",
		"xgboost_sparse_data_test_db": "",
		"cora":                        "",
		"scorecard":                   "",
	}
	for i := 0; i < len(resp); i++ {
		AssertContainsAny(a, expectedDBs, resp[i][0])
	}
}

func caseSelect(t *testing.T) {
	a := assert.New(t)
	cmd := fmt.Sprintf("select * from %s limit 2;", caseTrainTable)
	head, rows, _, err := connectAndRunSQL(cmd)
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	expectedHeads := []string{
		"sepal_length",
		"sepal_width",
		"petal_length",
		"petal_width",
		"class",
	}
	dialect := os.Getenv("SQLFLOW_TEST_DB")
	for idx, headCell := range head {
		if dialect == "hive" {
			a.Equal("train."+expectedHeads[idx], headCell)
		} else {
			a.Equal(expectedHeads[idx], headCell)
		}
	}
	expectedRows := [][]interface{}{
		{6.4, 2.8, 5.6, 2.2, int64(2)},
		{5.0, 2.3, 3.3, 1.0, int64(1)},
	}
	for rowIdx, row := range rows {
		for colIdx, rowCell := range row {
			a.True(EqualAny(expectedRows[rowIdx][colIdx], rowCell))
		}
	}

	if dialect == "mysql" || dialect == "hive" {
		describeSQL := fmt.Sprintf(`DESCRIBE %s;`, caseTrainTable)
		_, _, _, err := connectAndRunSQL(describeSQL)
		a.NoError(err)
	}
}

func caseCoverageCommon(t *testing.T) {
	cases := []string{
		`SELECT * FROM iris.train WHERE class<>2
TO TRAIN DNNClassifier
WITH
	model.n_classes = 2,
	model.hidden_units = [10, 10],
	train.batch_size = 4,
	validation.select = "SELECT * FROM iris.test WHERE class<>2",
	validation.metrics = "Accuracy,AUC",
	model.optimizer=RMSprop
LABEL class
INTO sqlflow_models.mytest_model;`, // train with metrics, with optimizer
		`SELECT * FROM iris.train WHERE class<>2
TO TRAIN sqlflow_models.DNNClassifier
WITH
	model.n_classes = 2,
	model.hidden_units = [10, 10],
	train.batch_size = 1,
	validation.select = "SELECT * FROM iris.test WHERE class<>2",
	validation.metrics = "Accuracy,AUC,Precision,Recall",
	model.optimizer=RMSprop, optimizer.learning_rate=0.1
LABEL class
INTO sqlflow_models.mytest_model;`, // train keras with metrics, with optimizer
		// TODO(shendiaomo): sqlflow_models.DNNClassifier.eval_metrics_fn only works when batch_size is 1
		`SELECT * FROM housing.train
TO TRAIN DNNRegressor
WITH
	model.hidden_units = [10, 10],
	train.batch_size = 4,
	validation.select = "SELECT * FROM housing.test",
	validation.metrics = "MeanAbsoluteError,MeanAbsolutePercentageError,MeanSquaredError"
LABEL target
INTO sqlflow_models.myreg_model;`, // train regression model with metrics
		`SELECT * FROM iris.train
TO TRAIN DNNLinearCombinedClassifier
WITH model.n_classes = 3, model.dnn_hidden_units = [10, 20], train.batch_size = 10, train.epoch = 2,
model.dnn_optimizer=RMSprop, dnn_optimizer.learning_rate=0.01
COLUMN sepal_length, sepal_width FOR linear_feature_columns
COLUMN petal_length, petal_width FOR dnn_feature_columns
LABEL class
INTO sqlflow_models.my_dnn_linear_model;`, // train deep wide model

	}
	a := assert.New(t)
	for _, sql := range cases {
		_, _, _, err := connectAndRunSQL(sql)
		a.NoError(err)
	}
}

func caseCoverageCustomModel(t *testing.T) {
	cases := []string{
		`SELECT * FROM iris.train
TO TRAIN sqlflow_models.DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20],
	 validation.select="select * from iris.test", validation.steps=2,
	 train.batch_size = 10, train.epoch=2
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;`, // custom model train
		`SELECT * FROM iris.test
TO PREDICT iris.predict.class
USING sqlflow_models.my_dnn_model;`, // custom model predict
		`SELECT * FROM iris.predict LIMIT 5;`, // get predict result
		`SELECT * FROM iris.train
TO TRAIN sqlflow_models.dnnclassifier_functional_model
WITH model.n_classes = 3, validation.metrics="CategoricalAccuracy"
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;`, // train functional keras model
		`SELECT * FROM iris.train
TO TRAIN sqlflow_models.AutoClassifier WITH model.n_classes = 3
LABEL class INTO sqlflow_models.my_adanet_model;`, // train adanet
		`SELECT * FROM iris.test LIMIT 10 TO EXPLAIN sqlflow_models.my_adanet_model;`, // explain adanet
	}
	a := assert.New(t)
	for _, sql := range cases {
		_, _, _, err := connectAndRunSQL(sql)
		a.NoError(err)
	}
}

func caseTrainRegression(t *testing.T) {
	seedEnvKey := "SQLFLOW_TF_RANDOM_SEED"
	os.Setenv(seedEnvKey, "1")
	defer os.Unsetenv(seedEnvKey)

	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT *
FROM housing.train
TO TRAIN LinearRegressor
WITH 
  model.label_dimension = 1, 
  train.batch_size = 16,
  train.epoch = 10
COLUMN f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13
LABEL target
INTO sqlflow_models.my_regression_model;`)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT *
FROM housing.test
TO PREDICT housing.predict.result
USING sqlflow_models.my_regression_model;`)
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run predSQL error: %v", err)
	}

	showPred := fmt.Sprintf(`SELECT *
FROM housing.predict LIMIT 5;`)
	_, rows, _, err := connectAndRunSQL(showPred)
	if err != nil {
		a.Fail("run showPred error: %v", err)
	}

	for _, row := range rows {
		// NOTE: predict result maybe random. Since it is
		// a regression model, the predict result may be
		// negative. Here we fix the TensorFlow random
		// seed to get the deterministic result.
		AssertGreaterEqualAny(a, row[13], float64(0))

		// avoiding nil features in predict result
		nilCount := 0
		for ; nilCount < 13 && row[nilCount] == nil; nilCount++ {
		}
		a.False(nilCount == 13)
	}
}

func caseTrainXGBoostRegressionConvergence(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`
SELECT * FROM housing.train
TO TRAIN xgboost.gbtree
WITH
	objective="reg:squarederror",
	scale_pos_weight=2,
	train.num_boost_round = 30,
	validation.select="SELECT * FROM housing.train LIMIT 20"
LABEL target
INTO sqlflow_models.my_xgb_regression_model;
`)
	_, _, messages, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}

	isConvergence := false
	reLog := regexp.MustCompile(`.*29.*train-rmse:(.+)?validate-rmse\:(.+)?`)
	for _, msg := range messages {
		sub := reLog.FindStringSubmatch(msg)
		if len(sub) == 3 {
			trainRmse, e := strconv.ParseFloat(strings.TrimSpace(sub[1]), 32)
			a.NoError(e)
			valRmse, e := strconv.ParseFloat(strings.TrimSpace(sub[2]), 32)
			a.NoError(e)
			a.Greater(trainRmse, 0.0)            // no overfitting
			a.LessOrEqual(trainRmse, 0.5)        // less the baseline
			a.GreaterOrEqual(valRmse, trainRmse) // verify the validation
			isConvergence = true
		}
	}
	a.Truef(isConvergence, strings.Join(messages, "\n"))

	evalSQL := fmt.Sprintf(`
SELECT * FROM housing.train
TO EVALUATE sqlflow_models.my_xgb_regression_model
WITH validation.metrics="mean_absolute_error,mean_squared_error"
LABEL target
INTO sqlflow_models.my_xgb_regression_model_eval_result;
`)
	_, _, messages, err = connectAndRunSQL(evalSQL)
	if err != nil {
		a.Fail("run evalSQL error: %v", err)
	}
}

func casePredictXGBoostRegression(t *testing.T) {
	a := assert.New(t)
	predSQL := fmt.Sprintf(`SELECT *
FROM housing.test
TO PREDICT housing.xgb_predict.target
USING sqlflow_models.my_xgb_regression_model;`)
	_, _, _, err := connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run predSQL error: %v", err)
	}

	showPred := fmt.Sprintf(`SELECT *
FROM housing.xgb_predict LIMIT 5;`)
	_, rows, _, err := connectAndRunSQL(showPred)
	if err != nil {
		a.Fail("run showPred error: %v", err)
	}

	for _, row := range rows {
		// NOTE: predict result maybe random, only check predicted
		// class >=0, need to change to more flexible checks than
		// checking expectedPredClasses := []int64{2, 1, 0, 2, 0}
		AssertGreaterEqualAny(a, row[13], float64(0))

		// avoiding nil features in predict result
		nilCount := 0
		for ; nilCount < 13 && row[nilCount] == nil; nilCount++ {
		}
		a.False(nilCount == 13)
	}
}

func caseShowTrain(t *testing.T) {
	driverName, _, _ := database.ParseURL(dbConnStr)
	if driverName != "mysql" && driverName != "hive" {
		t.Skip("Skipping non mysql/hive test.")
	}
	a := assert.New(t)
	trainSQL := `SELECT * FROM iris.train TO TRAIN xgboost.gbtree
	WITH objective="reg:squarederror"
	LABEL class 
	INTO sqlflow_models.my_xgb_model_for_show_train;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.FailNow("Train model failed: %v", err)
	}
	showSQL := `SHOW TRAIN sqlflow_models.my_xgb_model_for_show_train;`
	cols, _, _, err := connectAndRunSQL(showSQL)
	a.NoError(err)
	a.Equal(2, len(cols))
	a.Equal("Model", cols[0])
	a.Equal("Train Statement", cols[1])
}

func caseTrainSQL(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
	TO TRAIN DNNClassifier
	WITH
		model.n_classes = 3,
		model.hidden_units = [10, 20],
		validation.select = "SELECT * FROM %s LIMIT 30"
	COLUMN sepal_length, sepal_width, petal_length, petal_width
	LABEL class
	INTO %s;
	`, caseTrainTable, caseTrainTable, caseInto)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT * FROM %s
TO PREDICT %s.class
USING %s;`, caseTestTable, casePredictTable, caseInto)
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}

	showPred := fmt.Sprintf(`SELECT *
FROM %s LIMIT 5;`, casePredictTable)
	_, rows, _, err := connectAndRunSQL(showPred)
	if err != nil {
		a.Fail("Run showPred error: %v", err)
	}

	for _, row := range rows {
		// NOTE: predict result maybe random, only check predicted
		// class >=0, need to change to more flexible checks than
		// checking expectedPredClasses := []int64{2, 1, 0, 2, 0}
		AssertGreaterEqualAny(a, row[4], int64(0))

		// avoiding nil features in predict result
		nilCount := 0
		for ; nilCount < 4 && row[nilCount] == nil; nilCount++ {
		}
		a.False(nilCount == 4)
	}
}

var uniqueIDMutex sync.Mutex
var uniqueID = 0

func getUniqueID() int {
	uniqueIDMutex.Lock()
	defer uniqueIDMutex.Unlock()
	uniqueID++
	return uniqueID
}

// NOTE(sneaxiy): INDICATOR of XGBoost model does not support "TO EXPLAIN" yet
// We set skipExplain = true in INDICATOR unittest
func caseXGBoostFeatureColumnImpl(t *testing.T, table string, label string, selectColumns string, columnClauses string, nclasses int, nworkers int, isPai bool,
	skipExplain bool) {
	tableSplits := strings.SplitN(table, ".", 2)
	dbPrefix := ""
	if len(tableSplits) == 2 {
		dbPrefix = tableSplits[0] + "."
	}

	a := assert.New(t)
	if columnClauses != "" {
		columnClauses = "COLUMN " + columnClauses
	}

	trainSQLTemplate := `
	SELECT %s FROM %s TO TRAIN xgboost.gbtree
	WITH
		objective="multi:softprob",
		train.num_boost_round = 1,
		train.num_workers = %d,
		eta = 0.4,
		num_class = %d,
		validation.select="select %s from %s"
	%s
	LABEL %s
	INTO %s;`

	executeSQLFunc := func(sql string, shouldError bool) {
		if shouldError {
			connectAndRunSQLShouldError(sql)
			return
		}
		_, _, _, err := connectAndRunSQL(sql)
		a.NoError(err, fmt.Sprintf("SQL execution failure\n%s", sql))
	}

	dropModelTableFunc := func(table string) {
		executeSQLFunc(fmt.Sprintf("DROP TABLE IF EXISTS %s;", table), false)
	}

	hasModelTableFunc := func(table string) {
		_, rows, _, err := connectAndRunSQL(fmt.Sprintf("SELECT * FROM %s LIMIT 1;", table))
		a.NoError(err)
		a.Equal(len(rows), 1)
	}

	// a unique id to avoid name conflict when run parallel
	uniqueID := getUniqueID()

	var modelName string
	if isPai {
		modelName = fmt.Sprintf("xgb_fc_test_model_%d", uniqueID)
	} else {
		modelName = fmt.Sprintf("%sxgb_fc_test_model_%d", dbPrefix, uniqueID)
		dropModelTableFunc(modelName)
	}

	trainSQL := fmt.Sprintf(trainSQLTemplate, selectColumns, table, nworkers, nclasses, selectColumns, table, columnClauses, label, modelName)
	executeSQLFunc(trainSQL, false)
	if !isPai {
		hasModelTableFunc(modelName)
	}

	incrementalTrainSQLWithOverwriting := fmt.Sprintf(trainSQLTemplate, selectColumns, table, nworkers, nclasses, selectColumns, table,
		columnClauses,
		fmt.Sprintf("%s USING %s ", label, modelName), modelName)
	executeSQLFunc(incrementalTrainSQLWithOverwriting, false)
	if !isPai {
		hasModelTableFunc(modelName)
	}

	incrementalTrainSQLWithNotExist := fmt.Sprintf(trainSQLTemplate, selectColumns, table, nworkers, nclasses, selectColumns, table,
		columnClauses,
		fmt.Sprintf("%s USING %s ", label, modelName+"_none"), modelName)
	executeSQLFunc(incrementalTrainSQLWithNotExist, true)

	newModelName := modelName + "_new"
	if !isPai {
		dropModelTableFunc(newModelName)
	}
	incrementalTrainSQLWithoutOverwriting := fmt.Sprintf(trainSQLTemplate, selectColumns, table, nworkers, nclasses, selectColumns, table,
		columnClauses,
		fmt.Sprintf("%s USING %s ", label, modelName), newModelName)
	executeSQLFunc(incrementalTrainSQLWithoutOverwriting, false)
	if !isPai {
		hasModelTableFunc(modelName)
		hasModelTableFunc(newModelName)
	}

	modelName = newModelName

	predictTableName := fmt.Sprintf("%sxgb_fc_test_predict_table_%d", dbPrefix, uniqueID)
	predictSQL := fmt.Sprintf(`SELECT %s FROM %s TO PREDICT %s.%s_new USING %s;`, selectColumns, table, predictTableName, label, modelName)
	executeSQLFunc(predictSQL, false)

	if !isPai { // PAI does not support evaluate now
		evaluateTableName := fmt.Sprintf("%sxgb_fc_test_evaluate_table_%d", dbPrefix, uniqueID)
		evaluateSQL := fmt.Sprintf(`SELECT %s FROM %s TO EVALUATE %s WITH validation.metrics="accuracy_score" LABEL %s INTO %s;`,
			selectColumns, table, modelName, label, evaluateTableName)
		executeSQLFunc(evaluateSQL, false)
	}

	if !skipExplain {
		paiExplainExtra := ""
		if isPai {
			paiExplainExtra = fmt.Sprintf(`, label_col="%s" INTO %sxgb_fc_test_explain_table_%d`, label, dbPrefix, uniqueID)
		}
		explainSQL := fmt.Sprintf(`SELECT %s FROM %s TO EXPLAIN %s WITH summary.plot_type=bar %s;`, selectColumns, table, modelName, paiExplainExtra)
		executeSQLFunc(explainSQL, false)
	}

	if !isPai { // PAI does not support SHOW TRAIN, because the model is not saved into database
		showTrainSQL := fmt.Sprintf(`SHOW TRAIN %s;`, modelName)
		executeSQLFunc(showTrainSQL, false)
	}
}

// caseXGBoostFeatureColumn is cases to run xgboost e2e tests using feature columns
func caseXGBoostFeatureColumn(t *testing.T, isPai bool) {
	irisTrainTable := "iris.train"
	churnTrainTable := "churn.train"

	if isPai {
		irisTrainTable = caseDB + ".sqlflow_test_iris_train"
		churnTrainTable = caseDB + ".sqlflow_test_churn_train"
	}

	numWorkers := 1
	if isPai {
		numWorkers = 2
	}

	t.Run("CaseXGBoostNoFeatureColumn", func(*testing.T) {
		caseXGBoostFeatureColumnImpl(t, irisTrainTable, "class", "*", "", 3, numWorkers, isPai, false)
	})

	t.Run("CaseXGBoostBucketFeatureColumn", func(*testing.T) {
		caseXGBoostFeatureColumnImpl(t, irisTrainTable, "class", "*", "BUCKET(petal_length, [0, 1, 2, 3, 4, 5])", 3, numWorkers, isPai, false)
	})

	t.Run("CaseXGBoostCategoryFeatureColumn", func(*testing.T) {
		caseXGBoostFeatureColumnImpl(t, churnTrainTable, "seniorcitizen", "seniorcitizen, customerid, gender, tenure",
			`CATEGORY_HASH(customerid, 10), CATEGORY_ID(gender, 2)`, 2, numWorkers, isPai, false)
	})

	// NOTE(sneaxiy): INDICATOR of XGBoost model does not support "TO EXPLAIN" yet
	t.Run("CaseXGBoostCategoryFeatureColumnWithIndicator", func(*testing.T) {
		caseXGBoostFeatureColumnImpl(t, churnTrainTable, "seniorcitizen", "seniorcitizen, customerid, gender, tenure",
			`CATEGORY_HASH(customerid, 10), INDICATOR(CATEGORY_ID(gender, 2))`, 2, numWorkers, isPai, true)
	})
}

func caseTensorFlowIncrementalTrainImpl(t *testing.T, model string, isPai bool) {
	a := assert.New(t)

	executeSQLFunc := func(sql string) {
		_, _, _, err := connectAndRunSQL(sql)
		a.NoError(err, fmt.Sprintf("SQL execution failure\n%s", sql))
	}

	dropModelTableFunc := func(table string) {
		executeSQLFunc(fmt.Sprintf("DROP TABLE IF EXISTS %s;", table))
	}

	hasModelTableFunc := func(table string) {
		_, rows, _, err := connectAndRunSQL(fmt.Sprintf("SELECT * FROM %s LIMIT 1;", table))
		a.NoError(err)
		a.Equal(len(rows), 1)
	}

	trainTable := caseTrainTable
	db := strings.SplitN(trainTable, ".", 2)[0]

	modelSave := "tf_estimator_inc_train"
	if !isPai {
		modelSave = db + "." + modelSave
	}

	newModelSave := modelSave + "_new"
	if !isPai {
		dropModelTableFunc(modelSave)
		dropModelTableFunc(newModelSave)
	}

	trainSQL := fmt.Sprintf(`
	SELECT sepal_width, sepal_length, petal_width, petal_length, class FROM %s
	TO TRAIN %s
	WITH
		model.n_classes = 3,
		model.hidden_units = [10],
		validation.select = "SELECT * FROM %s"
	LABEL class
	INTO %s;
`, trainTable, model, trainTable, modelSave)

	executeSQLFunc(trainSQL)
	if !isPai {
		hasModelTableFunc(modelSave)
	}

	incTrainSQLTemplate := `
	SELECT sepal_width, sepal_length, petal_width, petal_length, class FROM %s
	TO TRAIN %s
	WITH 
		model.n_classes = 3,
		model.hidden_units = [10],
		validation.select = "SELECT * FROM %s"
	LABEL class
	USING %s
	INTO %s;
	`

	overwrittenIncTrainSQL := fmt.Sprintf(incTrainSQLTemplate, trainTable, model, trainTable, modelSave, modelSave)
	executeSQLFunc(overwrittenIncTrainSQL)
	if !isPai {
		hasModelTableFunc(modelSave)
	}

	notOverwrittenIncTrainSQL := fmt.Sprintf(incTrainSQLTemplate, trainTable, model, trainTable, modelSave, newModelSave)
	executeSQLFunc(notOverwrittenIncTrainSQL)
	if !isPai {
		hasModelTableFunc(modelSave)
		hasModelTableFunc(newModelSave)
	}

	predSQL := fmt.Sprintf(`SELECT * FROM %s TO PREDICT %s.tf_inc_train_pred.class USING %s;`,
		trainTable, db, newModelSave)
	executeSQLFunc(predSQL)
}

func caseTensorFlowIncrementalTrain(t *testing.T, isPai bool) {
	t.Run("CaseTensorFlowIncrementalTrainEstimator", func(t *testing.T) {
		caseTensorFlowIncrementalTrainImpl(t, "DNNClassifier", isPai)
	})

	if !isPai {
		t.Run("CaseTensorFlowIncrementalTrainKeras", func(t *testing.T) {
			caseTensorFlowIncrementalTrainImpl(t, "sqlflow_models.DNNClassifier", isPai)
		})
	}
}

func caseXGBoostSparseKeyValueColumn(t *testing.T) {
	a := assert.New(t)

	testDBType := os.Getenv("SQLFLOW_TEST_DB")
	dbName := "xgboost_sparse_data_test_db"
	isPai := os.Getenv("SQLFLOW_submitter") == "pai"
	const trainTable = "xgboost_sparse_data_train"

	if testDBType == "maxcompute" {
		dbName = caseDB
	}

	executeSQLFunc := func(sql string) {
		_, _, _, err := connectAndRunSQL(sql)
		a.NoError(err, fmt.Sprintf("SQL execution failure\n%s", sql))
	}

	hasModelTableFunc := func(table string) {
		_, rows, _, err := connectAndRunSQL(fmt.Sprintf("SELECT * FROM %s LIMIT 1;", table))
		if err != nil {
			a.FailNow("error: %s", err)
		}
		a.Equal(len(rows), 1)
	}

	trainedModel := "xgb_kv_column_trained_model"
	if !isPai {
		trainedModel = fmt.Sprintf("%s.%s", dbName, trainedModel)
	}

	const trainSQLTemplate = `SELECT c1, label_col FROM %s.%s
	TO TRAIN xgboost.gbtree
	WITH
		num_class = 3,
		objective = "multi:softprob",
		train.num_boost_round = 20
	COLUMN SPARSE(c1%s)
	LABEL label_col
	INTO %s;`

	trainSQL := fmt.Sprintf(trainSQLTemplate, dbName, trainTable, "", trainedModel)
	executeSQLFunc(trainSQL)
	if !isPai {
		hasModelTableFunc(trainedModel)
	}

	trainSQL = fmt.Sprintf(trainSQLTemplate, dbName, trainTable, ",11", trainedModel)
	executeSQLFunc(trainSQL)
	if !isPai {
		hasModelTableFunc(trainedModel)
	}

	const predictTable = "xgb_kv_column_predict_table"
	const predictSQLTemplate = `SELECT c1, label_col FROM %[1]s.%[2]s TO PREDICT %[1]s.%[3]s.%[4]s USING %[5]s;`

	predictSQLWithOriginalLabel := fmt.Sprintf(predictSQLTemplate, dbName, trainTable, predictTable, "new_label_col", trainedModel)
	executeSQLFunc(predictSQLWithOriginalLabel)
	columns, rows, _, err := connectAndRunSQL(fmt.Sprintf(`SELECT * FROM %s.%s;`, dbName, predictTable))
	a.NoError(err)
	a.Equal(3, len(rows))
	if isPai {
		// TODO(typhoonzero): currently sqlflowserver can not get the original train statement when predicting.
		// So the label used when training is not removed from the predict result table.
		// We can fix this when we move creating predicting table at Python runtime.
		a.Equal(3, len(columns))
		a.Equal("label_col", columns[1])
	} else {
		a.Equal(2, len(columns))
		columns = removeColumnNamePrefix(columns)
		a.Equal("new_label_col", columns[1])
	}
	a.Equal("c1", columns[0])

	predictSQLWithoutOriginalLabel := fmt.Sprintf(predictSQLTemplate, dbName, trainTable, predictTable, "label_col", trainedModel)
	executeSQLFunc(predictSQLWithoutOriginalLabel)
	columns, rows, _, err = connectAndRunSQL(fmt.Sprintf(`SELECT * FROM %s.%s;`, dbName, predictTable))
	a.NoError(err)
	a.Equal(3, len(rows))
	a.Equal(2, len(columns))
	columns = removeColumnNamePrefix(columns)
	a.Equal("c1", columns[0])
	a.Equal("label_col", columns[1])

	// PAI does not support TO EVALUATE yet
	if !isPai {
		const evaluateTable = "xgb_kv_column_evaluate_table"
		evaluateSQL := fmt.Sprintf(`SELECT c1, label_col FROM %[1]s.%[2]s 
TO EVALUATE %[3]s WITH validation.metrics="mean_squared_error" LABEL label_col INTO %[1]s.%[4]s;`, dbName, trainTable, trainedModel, evaluateTable)
		executeSQLFunc(evaluateSQL)
		explainSQL := fmt.Sprintf("SELECT c1, label_col FROM %s.%s TO EXPLAIN %s WITH summary.plot_type=bar;", dbName, trainTable, trainedModel)
		executeSQLFunc(explainSQL)
	}

	explainSQLTmpl := `SELECT c1, label_col FROM %[1]s.%[2]s
TO EXPLAIN %[3]s
WITH label_col=label_col
USING XGBoostExplainer
INTO %[1]s.sparse_xgb_explain_result;`
	explainSparseIntoSQL := fmt.Sprintf(explainSQLTmpl, dbName, trainTable, trainedModel)
	executeSQLFunc(explainSparseIntoSQL)
}

func decodeAnyTypedRowData(anyData [][]*any.Any) ([][]interface{}, error) {
	slice := make([][]interface{}, 0)
	for _, row := range anyData {
		rowSlice := make([]interface{}, 0)
		for _, cellValue := range row {
			decodedCellValue, err := proto.DecodePODType(cellValue)
			if err != nil {
				return nil, err
			}
			rowSlice = append(rowSlice, decodedCellValue)
		}
		slice = append(slice, rowSlice)
	}
	return slice, nil
}

func removeColumnNamePrefix(columns []string) []string {
	for i, c := range columns {
		split := strings.Split(c, ".")
		columns[i] = split[len(split)-1]
	}
	return columns
}

func caseTestOptimizeClauseWithoutGroupBy(t *testing.T) {
	a := assert.New(t)

	dbName := "optimize_test_db"
	resultTable := fmt.Sprintf("%s.%s", dbName, "woodcarving_result")

	woodcarvingOptimizeSQLTemplate := `SELECT * FROM optimize_test_db.woodcarving
TO MAXIMIZE SUM((price - materials_cost - other_cost) * %[1]s)
CONSTRAINT SUM(finishing * %[1]s) <= 100, SUM(carpentry * %[1]s) <= 80, %[1]s <= max_num
WITH 
	variables="%[1]s(product)",
	var_type="NonNegativeIntegers"
USING glpk
INTO ` + resultTable + `;`

	for _, product := range []string{"product", "amount"} {
		woodcarvingSQL := fmt.Sprintf(woodcarvingOptimizeSQLTemplate, product)
		_, _, _, err := connectAndRunSQL(woodcarvingSQL)
		a.NoError(err)

		actualResultValue := product
		if actualResultValue == "product" {
			actualResultValue += "_value"
		}

		queryResultSQL := fmt.Sprintf("SELECT product, %s FROM %s;", actualResultValue, resultTable)
		header, rows, _, err := connectAndRunSQL(queryResultSQL)
		header = removeColumnNamePrefix(header)
		a.NoError(err)
		a.Equal(2, len(header))

		a.Equal("product", header[0])
		a.Equal(actualResultValue, header[1])
		a.Equal(2, len(rows))
		decodedRows, err := decodeAnyTypedRowData(rows)
		a.NoError(err)
		a.Equal(len(rows), len(decodedRows))
		for i := 0; i < len(decodedRows); i++ {
			a.Equal(2, len(decodedRows[i]))
			a.IsType("", decodedRows[i][0])
			a.IsType(int64(0), decodedRows[i][1])
		}

		sort.Slice(decodedRows, func(i int, j int) bool {
			return decodedRows[i][0].(string) < decodedRows[j][0].(string)
		})

		a.True(reflect.DeepEqual(decodedRows[0], []interface{}{"soldier", int64(20)}))
		a.True(reflect.DeepEqual(decodedRows[1], []interface{}{"train", int64(60)}))
	}
}

func caseTestOptimizeClauseWithBinaryVarType(t *testing.T) {
	a := assert.New(t)

	dbName := "optimize_test_db"
	resultTable := fmt.Sprintf("%s.%s", dbName, "woodcarving_result")

	binaryWoodCarvingSQL := `SELECT * FROM optimize_test_db.woodcarving
TO MAXIMIZE SUM((price - materials_cost - other_cost) * amount)
CONSTRAINT SUM(finishing * amount) <= 100, SUM(carpentry * amount) <= 80, amount <= max_num
WITH 
	variables="amount(product)",
	var_type="Binary"
USING glpk
INTO ` + resultTable + `;`

	_, _, _, err := connectAndRunSQL(binaryWoodCarvingSQL)
	a.NoError(err)

	queryResultSQL := fmt.Sprintf("SELECT product, amount FROM %s;", resultTable)

	header, rows, _, err := connectAndRunSQL(queryResultSQL)
	header = removeColumnNamePrefix(header)
	a.NoError(err)
	a.Equal(2, len(header))

	a.Equal("product", header[0])
	a.Equal("amount", header[1])
	a.Equal(2, len(rows))
	decodedRows, err := decodeAnyTypedRowData(rows)
	a.NoError(err)
	a.Equal(len(rows), len(decodedRows))
	for i := 0; i < len(decodedRows); i++ {
		a.Equal(2, len(decodedRows[i]))
		a.IsType("", decodedRows[i][0])
		a.IsType(int64(0), decodedRows[i][1])
	}

	sort.Slice(decodedRows, func(i int, j int) bool {
		return decodedRows[i][0].(string) < decodedRows[j][0].(string)
	})

	a.True(reflect.DeepEqual(decodedRows[0], []interface{}{"soldier", int64(1)}))
	a.True(reflect.DeepEqual(decodedRows[1], []interface{}{"train", int64(1)}))
}

func caseTestOptimizeClauseWithoutConstraint(t *testing.T) {
	a := assert.New(t)

	dbName := "optimize_test_db"
	resultTable := fmt.Sprintf("%s.%s", dbName, "woodcarving_result")

	woodCarvingSQL := `SELECT * FROM optimize_test_db.woodcarving
TO MINIMIZE SUM((price - materials_cost - other_cost) * amount)
WITH 
	variables="amount(product)",
	var_type="PositiveIntegers"
USING glpk
INTO ` + resultTable + `;`

	_, _, _, err := connectAndRunSQL(woodCarvingSQL)
	a.NoError(err)

	queryResultSQL := fmt.Sprintf("SELECT product, amount FROM %s;", resultTable)

	header, rows, _, err := connectAndRunSQL(queryResultSQL)
	header = removeColumnNamePrefix(header)
	a.NoError(err)
	a.Equal(2, len(header))

	a.Equal("product", header[0])
	a.Equal("amount", header[1])
	a.Equal(2, len(rows))
	decodedRows, err := decodeAnyTypedRowData(rows)
	a.NoError(err)
	a.Equal(len(rows), len(decodedRows))
	for i := 0; i < len(decodedRows); i++ {
		a.Equal(2, len(decodedRows[i]))
		a.IsType("", decodedRows[i][0])
		a.IsType(int64(0), decodedRows[i][1])
	}

	sort.Slice(decodedRows, func(i int, j int) bool {
		return decodedRows[i][0].(string) < decodedRows[j][0].(string)
	})

	a.True(reflect.DeepEqual(decodedRows[0], []interface{}{"soldier", int64(1)}))
	a.True(reflect.DeepEqual(decodedRows[1], []interface{}{"train", int64(1)}))
}

func caseTestOptimizeClauseWithGroupBy(t *testing.T) {
	a := assert.New(t)

	dbName := "optimize_test_db"
	resultTable := fmt.Sprintf("%s.%s", dbName, "shipment_result")

	shipmentOptimizeSQL := fmt.Sprintf(`SELECT 
		t.plants AS plants, 
		t.markets AS markets, 
		t.distance AS distance, 
		p.capacity AS capacity, 
		m.demand AS demand FROM optimize_test_db.transportation_table AS t
    LEFT JOIN optimize_test_db.plants_table AS p ON t.plants = p.plants
    LEFT JOIN optimize_test_db.markets_table AS m ON t.markets = m.markets
	TO MINIMIZE SUM(shipment * distance * 90 / 1000)
	CONSTRAINT SUM(shipment) <= capacity GROUP BY plants,
			   SUM(shipment) >= demand GROUP BY markets
	WITH variables="shipment(plants,markets)",
		 var_type="NonNegativeIntegers"
	INTO %s;`, resultTable)

	_, _, _, err := connectAndRunSQL(shipmentOptimizeSQL)
	a.NoError(err)

	queryResultSQL := fmt.Sprintf("SELECT plants, markets, shipment FROM %s;", resultTable)
	header, rows, _, err := connectAndRunSQL(queryResultSQL)
	header = removeColumnNamePrefix(header)
	a.NoError(err)
	a.Equal(3, len(header))

	a.Equal("plants", header[0])
	a.Equal("markets", header[1])
	a.Equal("shipment", header[2])

	a.Equal(4, len(rows))
	decodedRows, err := decodeAnyTypedRowData(rows)
	a.NoError(err)
	a.Equal(len(rows), len(decodedRows))
	for i := 0; i < len(decodedRows); i++ {
		a.Equal(3, len(decodedRows[i]))
		a.IsType("", decodedRows[i][0])
		a.IsType("", decodedRows[i][1])
		a.IsType(int64(0), decodedRows[i][2])
	}

	sort.Slice(decodedRows, func(i int, j int) bool {
		if decodedRows[i][0].(string) != decodedRows[j][0].(string) {
			return decodedRows[i][0].(string) < decodedRows[j][0].(string)
		}
		return decodedRows[i][1].(string) < decodedRows[j][1].(string)
	})

	a.True(reflect.DeepEqual(decodedRows[0], []interface{}{"plantA", "marketA", int64(100)}))
	a.True(reflect.DeepEqual(decodedRows[1], []interface{}{"plantA", "marketB", int64(0)}))
	a.True(reflect.DeepEqual(decodedRows[2], []interface{}{"plantB", "marketA", int64(30)}))
	a.True(reflect.DeepEqual(decodedRows[3], []interface{}{"plantB", "marketB", int64(60)}))
}

func caseEnd2EndXGBoostDenseFeatureColumn(t *testing.T, isPai bool) {
	trainTableName := "feature_derivation_case.train"
	modelName := "feature_derivation_case.xgb_dense_column_model"
	predictTableName := "feature_derivation_case.xgb_dense_column_predict_table"
	evaluateTableName := "feature_derivation_case.xgb_dense_column_evaluate_table"

	if isPai {
		trainTableName = caseDB + ".feature_derivation_train"
		modelName = "my_xgb_dense_column_model"
		predictTableName = caseDB + ".xgb_dense_column_predict_table"
		evaluateTableName = caseDB + ".xgb_dense_column_evaluate_table"
	}

	sqlTemplate := `SELECT c3, class FROM %[1]s
TO TRAIN xgboost.gbtree
WITH objective="binary:logistic", 
    validation.select="SELECT c3, class FROM %[1]s", 
    train.num_boost_round=100,
    eta=0.3,
    max_depth=5
column DENSE(c3, 4)
LABEL class
INTO %[2]s;

SELECT c3 FROM %[1]s TO PREDICT %[3]s.class USING %[2]s;

SELECT * FROM %[3]s;

SELECT c3, class FROM %[1]s
TO EVALUATE %[2]s
WITH
	validation.metrics="accuracy_score,f1_score"
LABEL class
INTO %[4]s; 

SELECT * FROM %[4]s;`

	const selectTrainTableSQL = `SELECT * FROM %[2]s;`

	if !isPai {
		sqlTemplate += selectTrainTableSQL
	}

	sqls := fmt.Sprintf(sqlTemplate, trainTableName, modelName, predictTableName, evaluateTableName)

	a := assert.New(t)
	for _, sql := range strings.Split(sqls, ";") {
		sql := strings.TrimSpace(sql)
		if sql == "" {
			continue
		}

		sql += ";"
		_, _, _, err := connectAndRunSQL(sql)
		if err != nil {
			a.Fail(fmt.Sprintf("Run SQL failure:\n%s\n%s", sql, err.Error()))
		}
	}
}

func caseWeightedKeyValueColumn(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s.weighted_key_value_train
TO TRAIN DNNClassifier
WITH model.hidden_units=[64,32], train.batch_size=2
COLUMN EMBEDDING(
	WEIGHTED_CATEGORY(CATEGORY_HASH(SPARSE(feature, 10, ",", "int", ":", "float"), 10)),
	32
)
LABEL label_col
INTO %s;`, caseDB, caseInto)
	_, _, _, err := connectAndRunSQL(trainSQL)
	a.NoError(err)

	predSQL := fmt.Sprintf(`SELECT * FROM %[1]s.weighted_key_value_train
TO PREDICT %[1]s.weighted_key_value_pred_result.label_col
USING %[2]s;`, caseDB, caseInto)
	_, _, _, err = connectAndRunSQL(predSQL)
	a.NoError(err)
}
