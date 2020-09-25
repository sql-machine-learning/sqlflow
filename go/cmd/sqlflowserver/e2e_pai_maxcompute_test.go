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
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	server "sqlflow.org/sqlflow/go/sqlflowserver"
)

func CasePAIMaxComputeTrainPredictCategoricalFeature(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	trainSQL := `SELECT cast(sepal_length as int) sepal_length, class
FROM alifin_jtest_dev.sqlflow_test_iris_train
TO TRAIN DNNClassifier WITH
		model.hidden_units = [10, 20], model.n_classes=3
COLUMN EMBEDDING(CATEGORY_ID(sepal_length, 1000), 2, "sum")
LABEL class
INTO e2etest_predict_categorical_feature;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := `SELECT cast(sepal_length as int) sepal_length, class FROM alifin_jtest_dev.sqlflow_test_iris_test
TO PREDICT alifin_jtest_dev.pred_catcol.class USING e2etest_predict_categorical_feature;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}

	trainSQL = `SELECT cast(sepal_length as int) sepal_length, cast(sepal_width as int) sepal_width, class
FROM alifin_jtest_dev.sqlflow_test_iris_train
TO TRAIN DNNLinearCombinedClassifier WITH
		model.dnn_hidden_units = [10, 20], model.n_classes=3
COLUMN EMBEDDING(CATEGORY_ID(sepal_length, 20), 2, "sum") for dnn_feature_columns
COLUMN EMBEDDING(CATEGORY_ID(sepal_width, 20), 2, "sum") for linear_feature_columns
LABEL class
INTO e2etest_predict_categorical_feature2;`
	_, _, _, err = connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL = `SELECT cast(sepal_length as int) sepal_length, cast(sepal_width as int) sepal_width, class FROM alifin_jtest_dev.sqlflow_test_iris_test
TO PREDICT alifin_jtest_dev.pred_catcol2.class USING e2etest_predict_categorical_feature2;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}
}

func CasePAIMaxComputeTrainDistributed(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
TO TRAIN DNNClassifier
WITH
	model.n_classes = 3,
	model.hidden_units = [10, 20],
	train.num_workers=2,
	train.num_ps=2,
	train.save_checkpoints_steps=20,
	train.epoch=10,
	train.batch_size=4,
	train.verbose=1
LABEL class
INTO e2etest_dnn_model_distributed;`, caseTrainTable)
	connectAndRunSQLShouldError(trainSQL)

	trainSQL = fmt.Sprintf(`SELECT * FROM %s
TO TRAIN DNNClassifier
WITH
	model.n_classes = 3,
	model.hidden_units = [10, 20],
	train.num_workers=2,
	train.num_ps=2,
	train.save_checkpoints_steps=20,
	train.epoch=10,
	train.batch_size=4,
	train.verbose=1,
	validation.select="select * from %s"
LABEL class
INTO e2etest_dnn_model_distributed;`, caseTrainTable, caseTestTable)
	_, _, _, err := connectAndRunSQL(trainSQL)
	a.NoError(err)
}

func CasePAIMaxComputeTrainDistributedKeras(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
TO TRAIN sqlflow_models.dnnclassifier_functional_model
WITH
	model.n_classes=3,
	train.num_workers=2,
	train.num_ps=2,
	train.epoch=10,
	train.batch_size=4,
	train.verbose=1,
	validation.select="select * from %s",
	validation.metrics="CategoricalAccuracy"
LABEL class
INTO e2etest_keras_dnn_model_distributed;`, caseTrainTable, caseTestTable)
	_, _, _, err := connectAndRunSQL(trainSQL)
	a.NoError(err)
}

func CasePAIMaxComputeTrainPredictDiffColumns(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT sepal_length, sepal_width, class FROM %s
TO TRAIN DNNClassifier
WITH model.hidden_units=[64,32], model.n_classes=3, train.batch_size=4
LABEL class 
INTO e2etest_selected_cols_model;
`, caseTrainTable)
	_, _, _, e := connectAndRunSQL(trainSQL)
	a.NoError(e, "run trainSQL error.")

	predSQL := fmt.Sprintf(`SELECT * FROM %s
	TO PREDICT %s.e2etest_selected_cols_pred.target
	USING e2etest_selected_cols_model;
		`, caseTestTable, caseDB)
	_, _, _, e = connectAndRunSQL(predSQL)
	a.NoError(e, "run predSQL error")

	query := fmt.Sprintf(`SELECT * FROM %s.e2etest_selected_cols_pred LIMIT 1;`, caseDB)
	_, resultRows, _, e := connectAndRunSQL(query)
	a.NoError(e)

	query = fmt.Sprintf(`SELECT * FROM %s LIMIT 1;`, caseTestTable)
	_, predRows, _, e := connectAndRunSQL(query)
	a.NoError(e)
	for idx := range resultRows {
		a.Equal(predRows[0][idx], resultRows[0][idx])
	}
}

func CasePAIMaxComputeTrainXGBDistributed(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
	TO TRAIN xgboost.gbtree
	WITH
		objective="multi:softprob",
		train.num_boost_round = 30,
		train.num_workers = 2,
		eta = 0.4,
		num_class = 3,
		train.batch_size=10,
		validation.select="select * from %s"
	LABEL class
	INTO e2etest_xgb_classi_model;`, caseTrainTable, caseTrainTable)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}
}

func CasePAIMaxComputeTrainTFBTDistributed(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s WHERE class < 2
TO TRAIN BoostedTreesClassifier
WITH
	model.center_bias=True,
	model.n_batches_per_layer=70,
	train.num_workers=2,
	train.num_ps=1,
	train.epoch=10,
	validation.select="select * from %s"
LABEL class
INTO e2etest_tfbt_model_distributed;`, caseTrainTable, caseTestTable)
	_, _, _, err := connectAndRunSQL(trainSQL)
	a.NoError(err)
}

func CaseTrainPAIKMeans(t *testing.T) {
	a := assert.New(t)
	err := dropPAIModel(dbConnStr, caseInto)
	a.NoError(err)

	trainSQL := fmt.Sprintf(`SELECT * FROM %s
	TO TRAIN kmeans 
	WITH
		center_count=3,
		idx_table_name=%s,
		excluded_columns=class
	INTO %s;
	`, caseTrainTable, caseTrainTable+"_test_output_idx", caseInto)
	_, _, _, err = connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT * FROM %s
	TO PREDICT %s.cluster_index
	USING %s;
	`, caseTestTable, casePredictTable, caseInto)
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}
}

func dropPAIModel(dataSource, modelName string) error {
	code := fmt.Sprintf(`import subprocess
import runtime.db
driver, dsn = "%s".split("://")
assert driver == "maxcompute"
user, passwd, address, database = runtime.db.parseMaxComputeDSN(dsn)
cmd = "drop offlinemodel if exists %s"
subprocess.run(["odpscmd", "-u", user,
                           "-p", passwd,
                           "--project", database,
                           "--endpoint", address,
                           "-e", cmd],
               check=True)	
	`, dataSource, modelName)
	cmd := exec.Command("python", "-u")
	cmd.Stdin = bytes.NewBufferString(code)
	if e := cmd.Run(); e != nil {
		return e
	}
	return nil
}

func CaseTrainPAIRandomForests(t *testing.T) {
	a := assert.New(t)
	err := dropPAIModel(dbConnStr, "my_rf_model")
	a.NoError(err)

	trainSQL := fmt.Sprintf(`SELECT * FROM %s
TO TRAIN randomforests
WITH tree_num = 3
LABEL class
INTO my_rf_model;`, caseTrainTable)
	_, _, _, err = connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT * FROM %s
TO PREDICT %s.class
USING my_rf_model;`, caseTestTable, casePredictTable)
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	explainSQL := fmt.Sprintf(`SELECT * FROM %s
TO EXPLAIN my_rf_model
WITH label_column = class
USING TreeExplainer
INTO %s.rf_model_explain;`, caseTestTable, caseDB)
	_, _, _, err = connectAndRunSQL(explainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}
}

func CasePAIMaxComputeDNNTrainPredictExplain(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
TO TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20],
optimizer.learning_rate=0.01, model.optimizer="AdagradOptimizer"
LABEL class
INTO e2etest_pai_dnn;`, caseTrainTable)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	evalSQL := fmt.Sprintf(`SELECT * FROM %s
TO EVALUATE e2etest_pai_dnn
WITH validation.metrics="Accuracy,Recall"
LABEL class
INTO %s.e2etest_pai_dnn_evaluate_result;`, caseTrainTable, caseDB)
	_, _, _, err = connectAndRunSQL(evalSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT * FROM %s
TO PREDICT %s.pai_dnn_predict.class
USING e2etest_pai_dnn;`, caseTestTable, caseDB)
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}

	showPred := fmt.Sprintf(`SELECT *
FROM %s.pai_dnn_predict LIMIT 5;`, caseDB)
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

	explainSQL := fmt.Sprintf(`SELECT * FROM %s
TO EXPLAIN e2etest_pai_dnn
WITH label_col=class
USING TreeExplainer
INTO %s.pai_dnn_explain_result;`, caseTestTable, caseDB)
	_, _, _, err = connectAndRunSQL(explainSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}
}

func CasePAIMaxComputeTrainDenseCol(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	// Test train and predict using concated columns sepal_length, sepal_width, petal_length, petal_width
	trainSQL := fmt.Sprintf(`SELECT class, CONCAT(sepal_length, ",", sepal_width, ",", petal_length, ",", petal_width) AS f1
FROM %s
TO TRAIN DNNClassifier
WITH model.hidden_units=[64,32], model.n_classes=3, train.batch_size=32
COLUMN DENSE(f1, 4)
LABEL class
INTO e2etest_dense_input;`, caseTrainTable)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}
}

func CasePAIMaxComputeTrainDenseColWithoutIndicatingShape(t *testing.T) {
	t.Parallel()
	a := assert.New(t)

	trainSQL := fmt.Sprintf(`SELECT class, sepal_length, sepal_width, petal_length, petal_width
FROM %s
TO TRAIN DNNClassifier WITH model.hidden_units=[64,32], model.n_classes=3, train.batch_size=32
COLUMN DENSE(sepal_length)
LABEL class
INTO e2etest_dense_input_without_indicating_shape;`, caseTrainTable)

	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL without indicating shape error: %v", err)
	}
}

func CasePAIMaxComputeTrainXGBoost(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	// train with batch_size to split dmatrix files
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
TO TRAIN xgboost.gbtree
WITH
	objective="multi:softmax",
	train.num_boost_round = 30,
	eta = 0.4,
	num_class = 3,
	train.batch_size=10,
	validation.select="select * from %s"
LABEL class
INTO e2etest_xgb_classi_model;`, caseTrainTable, caseTestTable)
	_, _, _, err := connectAndRunSQL(trainSQL)
	a.NoError(err, "Run trainSQL error.")
	// train without batch_size
	trainSQL = fmt.Sprintf(`SELECT * FROM %s
TO TRAIN xgboost.gbtree
WITH
	objective="multi:softmax",
	train.num_boost_round = 30,
	eta = 0.4,
	num_class = 3,
	validation.select="select * from %s"
LABEL class
INTO e2etest_xgb_classi_model;`, caseTrainTable, caseTestTable)
	_, _, _, err = connectAndRunSQL(trainSQL)
	a.NoError(err, "Run trainSQL error.")

	predSQL := fmt.Sprintf(`SELECT * FROM %s
TO PREDICT %s.pai_xgb_predict.class
WITH
	predict.num_workers=2
USING e2etest_xgb_classi_model;`, caseTestTable, caseDB)
	_, _, _, err = connectAndRunSQL(predSQL)
	a.NoError(err, "Run predSQL error.")

	evalSQL := fmt.Sprintf(`SELECT * FROM %s
TO EVALUATE e2etest_xgb_classi_model
WITH validation.metrics="accuracy_score"
LABEL class
INTO %s.e2etest_xgb_evaluate_result;`, caseTestTable, caseDB)
	_, _, _, err = connectAndRunSQL(evalSQL)
	a.NoError(err, "Run evalSQL error.")

	titanicTrain := fmt.Sprintf(`SELECT * FROM %s.sqlflow_titanic_train
TO TRAIN xgboost.gbtree
WITH objective="binary:logistic"
LABEL survived
INTO e2etest_xgb_titanic;`, caseDB)
	_, _, _, err = connectAndRunSQL(titanicTrain)
	a.NoError(err, "Run titanicTrain error.")

	titanicExplain := fmt.Sprintf(`SELECT * FROM %s.sqlflow_titanic_train
TO EXPLAIN e2etest_xgb_titanic
WITH label_col=survived
INTO %s.e2etest_titanic_explain_result;`, caseDB, caseDB)
	_, _, _, err = connectAndRunSQL(titanicExplain)
	a.NoError(err, "Run titanicExplain error.")
}

func CasePAIMaxComputeTrainCustomModel(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
TO TRAIN sqlflow_models.DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20], validation.select="select * from %s", validation.steps=2
LABEL class
INTO e2etest_keras_dnn;`, caseTrainTable, caseTestTable)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT * FROM %s
TO PREDICT %s.keras_predict.class
USING e2etest_keras_dnn;`, caseTestTable, caseDB)
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run predSQL error: %v", err)
	}
}

func CasePAIMaxComputeWeightedCategory(t *testing.T) {
	t.Parallel()
	a := assert.New(t)

	trainSQL := `SELECT * FROM alifin_jtest_dev.weighted_key_value_train
TO TRAIN DNNClassifier
WITH model.n_classes = 2, model.hidden_units = [64,32],train.batch_size=128,train.epoch=2
COLUMN EMBEDDING(WEIGHTED_CATEGORY(CATEGORY_HASH(SPARSE(feature, 128, ",", "int", ":", "float"), 128)), 32)
LABEL label_col
INTO e2etest_weighted_emb;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}

	predSQL := `SELECT * FROM alifin_jtest_dev.weighted_key_value_train
TO PREDICT alifin_jtest_dev.weighted_emb.label_col
USING e2etest_weighted_emb;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run predSQL error: %v", err)
	}
}

// TestEnd2EndMaxComputePAI test cases that runs on PAI. Need to set below
// environment variables to run the test:
// SQLFLOW_submitter=pai
// SQLFLOW_TEST_DB=maxcompute
// SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT="xxx"
// SQLFLOW_TEST_DB_MAXCOMPUTE_ENDPOINT="xxx"
// SQLFLOW_TEST_DB_MAXCOMPUTE_AK="xxx"
// SQLFLOW_TEST_DB_MAXCOMPUTE_SK="xxx"
// SQLFLOW_OSS_CHECKPOINT_CONFIG="xxx"
// SQLFLOW_OSS_ENDPOINT="xxx"
// SQLFLOW_OSS_AK="xxx"
// SQLFLOW_OSS_SK="xxx"
func TestEnd2EndMaxComputePAI(t *testing.T) {
	testDBDriver := os.Getenv("SQLFLOW_TEST_DB")
	if testDBDriver != "maxcompute" {
		t.Skip("Skipping non maxcompute tests")
	}
	if os.Getenv("SQLFLOW_submitter") != "pai" && os.Getenv("SQLFLOW_submitter") != "pai_local" {
		t.Skip("Skip non PAI tests")
	}
	AK := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_AK")
	SK := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_SK")
	endpoint := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_ENDPOINT")
	dbConnStr = fmt.Sprintf("maxcompute://%s:%s@%s", AK, SK, endpoint)
	modelDir := ""

	tmpDir, caCrt, caKey, err := generateTempCA()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatalf("failed to generate CA pair %v", err)
	}

	caseDB = os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
	if caseDB == "" {
		t.Fatalf("Must set env SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
	}
	caseTrainTable = caseDB + ".sqlflow_test_iris_train"
	caseTestTable = caseDB + ".sqlflow_test_iris_test"
	casePredictTable = caseDB + ".sqlflow_test_iris_predict"
	// write model to current MaxCompute project
	caseInto = "my_dnn_model"

	if err := prepareTestData(dbConnStr); err != nil {
		t.FailNow()
	}

	go start(modelDir, caCrt, caKey, unitTestPort, false)
	server.WaitPortReady(fmt.Sprintf("localhost:%d", unitTestPort), 0)

	t.Run("group", func(t *testing.T) {
		t.Run("CasePAIMaxComputeDNNTrainPredictExplain", CasePAIMaxComputeDNNTrainPredictExplain)
		t.Run("CasePAIMaxComputeTrainDenseCol", CasePAIMaxComputeTrainDenseCol)
		t.Run("CasePAIMaxComputeTrainDenseColWithoutIndicatingShape", CasePAIMaxComputeTrainDenseColWithoutIndicatingShape)
		t.Run("CasePAIMaxComputeTrainXGBoost", CasePAIMaxComputeTrainXGBoost)
		t.Run("CasePAIMaxComputeTrainCustomModel", CasePAIMaxComputeTrainCustomModel)
		t.Run("CasePAIMaxComputeTrainDistributed", CasePAIMaxComputeTrainDistributed)
		t.Run("CasePAIMaxComputeTrainPredictCategoricalFeature", CasePAIMaxComputeTrainPredictCategoricalFeature)
		t.Run("CasePAIMaxComputeTrainTFBTDistributed", CasePAIMaxComputeTrainTFBTDistributed)
		t.Run("CasePAIMaxComputeTrainDistributedKeras", CasePAIMaxComputeTrainDistributedKeras)
		t.Run("CasePAIMaxComputeTrainPredictDiffColumns", CasePAIMaxComputeTrainPredictDiffColumns)
		t.Run("CasePAIMaxComputeTrainXGBDistributed", CasePAIMaxComputeTrainXGBDistributed)
		// FIXME(typhoonzero): Add this test back when we solve error: model already exist issue on the CI.
		// t.Run("CaseTrainPAIRandomForests", CaseTrainPAIRandomForests)
		t.Run("CaseXGBoostSparseKeyValueColumn", caseXGBoostSparseKeyValueColumn)
		t.Run("CaseEnd2EndXGBoostDenseFeatureColumn", func(t *testing.T) {
			caseEnd2EndXGBoostDenseFeatureColumn(t, true)
		})

		t.Run("CasePAIMaxComputeWeightedCategory", CasePAIMaxComputeWeightedCategory)
	})
}
