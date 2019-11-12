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

package alps

import (
	"bytes"
	"fmt"

	"os"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	pb "sqlflow.org/sqlflow/pkg/server/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen"
)

func mockTrainIR() *codegen.TrainIR {
	cfg := &mysql.Config{
		User:                 "root",
		Passwd:               "root",
		Net:                  "tcp",
		Addr:                 "127.0.0.1:3306",
		AllowNativePasswords: true,
	}

	_ = `SELECT dense, deep, wide FROM kaggle_credit_fraud_training_data 
	TRAIN DNNLinearCombinedClassifier 
	WITH 
		model.dnn_hidden_units = [10, 20],
		train.max_steps = 1000,
		engine.type = "yarn"
	COLUMN
		DENSE(dense, 5, comma),
		SPARSE(deep, 2000, comma),
		NUMERIC(dense, 5),
		EMBEDDING(CATEGORY_ID(deep, 2000), 8, mean),
		EMBEDDING(CATEGORY_ID(deep, 500), 8, mean) FOR dnn_feature_columns
	COLUMN
		SPARSE(wide, 1000, comma),
		EMBEDDING(CATEGORY_ID(wide, 1000), 16, mean) FOR linear_feature_columns
	LABEL c3
	INTO model_table;`
	wide_embedding := &codegen.EmbeddingColumn{&codegen.CategoryIDColumn{&codegen.FieldMeta{"wide", codegen.Int, ",", []int{1000}, true, nil, 0}, 1000}, 16, "mean", "", ""}
	deep_embedding := &codegen.EmbeddingColumn{&codegen.CategoryIDColumn{&codegen.FieldMeta{"deep", codegen.Int, ",", []int{2000}, true, nil, 0}, 2000}, 8, "mean", "", ""}
	return &codegen.TrainIR{
		DataSource:       fmt.Sprintf("mysql://%s", cfg.FormatDSN()),
		Select:           "SELECT dense, deep, wide FROM kaggle_credit_fraud_training_data;",
		ValidationSelect: "SELECT dense, deep, wide FROM kaggle_credit_fraud_testing_data;",
		Estimator:        "DNNLinearCombinedClassifier",
		Attributes: map[string]interface{}{
			"engine.type":        "yarn",
			"train.max_steps":    1000,
			"model.dnn_hidden_units": []int{10, 20}},
		Features: map[string][]codegen.FeatureColumn{
			"dnn_feature_columns": {
				&codegen.NumericColumn{&codegen.FieldMeta{"dense", codegen.Float, ",", []int{5}, false, nil, 0}},
				deep_embedding},
			"linear_feature_columns": {wide_embedding}},
		Save: "model_table",
		Label: &codegen.NumericColumn{&codegen.FieldMeta{"c3", codegen.Int, ",", []int{1}, false, nil, 0}}}
}

func TestTrainALPSFiller(t *testing.T) {
	a := assert.New(t)
	tir := mockTrainIR()
	session := &pb.Session{UserId: "sqlflow_user"}
	
	filler, e := newALPSTrainFillerWithIR(tir, nil, session)
	a.NoError(e)

	a.True(filler.IsTraining)
	a.Equal("kaggle_credit_fraud_training_data", filler.TrainInputTable)
	a.True(strings.Contains(filler.X, "SparseColumn(name=\"deep\", shape=[2000], dtype=\"int\")"), filler.X)
	a.True(strings.Contains(filler.X, "SparseColumn(name=\"wide\", shape=[1000], dtype=\"int\")"), filler.X)
	a.True(strings.Contains(filler.X, "DenseColumn(name=\"dense\", shape=[5], dtype=\"float\", separator=\",\")"), filler.X)
	a.Equal("DenseColumn(name=\"c3\", shape=[1], dtype=\"int\", separator=\",\")", filler.Y)
	a.True(strings.Contains(filler.ModelCreatorCode, "tf.estimator.DNNLinearCombinedClassifier(dnn_hidden_units=[10,20]"), filler.ModelCreatorCode)
	a.Equal(1000, filler.TrainClause.MaxSteps)
	a.Equal(filler.ModelDir, "arks://sqlflow/sqlflow_user/model_table.tar.gz")
	a.Equal(filler.UserID, "sqlflow_user")
	var program bytes.Buffer
	alpsTrainTemplate.Execute(&program, filler)
	// code := program.String()
	// fmt.Println(code)

	
}

func mockPredIR(trainIR *codegen.TrainIR) *codegen.PredictIR {
	return &codegen.PredictIR{
		DataSource:  trainIR.DataSource,
		Select:  `SELECT predict_fun(concat(",", col_1, col_2)) AS (info, score) FROM db.table;`,
		ResultTable: "db.predict_result",
		ResultColumn: "score",
		Attributes:  make(map[string]interface{}),
		TrainIR:     trainIR,
	}
}

func TestPredALPSFiller(t *testing.T) {
	a := assert.New(t)
	os.Setenv("OSS_KEY", "sqlflow_key")
	os.Setenv("OSS_ID", "sqlflow_id")
	os.Setenv("OSS_ENDPOINT", "http://sqlflow-oss-endpoint")

	tir := mockTrainIR()
	pir := mockPredIR(tir)
	session := &pb.Session{UserId: "sqlflow_user"}

	os.Setenv("OSS_KEY", "sqlflow_key")
	os.Setenv("OSS_ID", "sqlflow_id")
	os.Setenv("OSS_ENDPOINT", "http://sqlflow-oss-endpoint")

	filler, e := newALPSPredictFillerWithIR(pir, session)
	a.NoError(e)

	a.False(filler.IsTraining)
	a.Equal(filler.PredictInputTable, "db.table")
	a.Equal(filler.PredictOutputTable, "db.predict_result")
	a.Equal(filler.PredictUDF, `SELECT predict_fun(concat(",", col_1, col_2)) AS (info, score) FROM db.table;`)
	a.Equal(filler.ModelDir, "oss://cmps-model/sqlflow/sqlflow_user/model_table.tar.gz")
	a.Equal(filler.UserID, "sqlflow_user")
	a.Equal(filler.OSSID, "sqlflow_id")
	a.Equal(filler.OSSKey, "sqlflow_key")

	var program bytes.Buffer
	e = alpsPredTemplate.Execute(&program, filler)
	// code2 := program2.String()
	// fmt.Println(code2)
	a.NoError(e)

	arr := strings.Split(program.String(), ";")
	udfSQL := strings.Trim(arr[len(arr)-2], "\n")
	a.Equal(udfSQL,
		`CREATE TABLE IF NOT EXISTS db.predict_result AS `+
			`SELECT predict_fun(concat(",", col_1, col_2)) AS (info, score) FROM db.table`)
}


func TestTrainALPSEmbeddingInitializer(t *testing.T) {
	a := assert.New(t)

	tir := mockTrainIRALPSEmbeddingInitializer()
	session := &pb.Session{UserId: "sqlflow_user"}

	filler, e := newALPSTrainFillerWithIR(tir, nil, session)
	a.NoError(e)
	a.True(strings.Contains(filler.FeatureColumnCode, "tf.feature_column.embedding_column(tf.feature_column.categorical_column_with_identity(key=\"deep\", num_buckets=2000), dimension=8, combiner=\"sum\", initializer=tf.random_normal_initializer(stddev=0.001))"))
}

func mockTrainIRALPSEmbeddingInitializer() *codegen.TrainIR{


	_ = `SELECT deep FROM kaggle_credit_fraud_training_data 
		TO TRAIN DNNClassifier 
		WITH 
			model.dnn_hidden_units = [10, 20],
			train.max_steps = 1000,
			engine.type = "yarn"
		COLUMN
			SPARSE(deep, 2000, comma),
			EMBEDDING(CATEGORY_ID(deep, 2000), 8, "sum", "tf.random_normal_initializer(stddev=0.001)") FOR dnn_feature_columns
		LABEL class
		INTO model_table;`

	cfg := &mysql.Config{
		User:                 "root",
		Passwd:               "root",
		Net:                  "tcp",
		Addr:                 "127.0.0.1:3306",
		AllowNativePasswords: true,
	}
	deepEmbedding := &codegen.EmbeddingColumn{&codegen.CategoryIDColumn{&codegen.FieldMeta{"deep", codegen.Int, ",", []int{2000}, true, nil, 0}, 2000}, 8, "sum", "tf.random_normal_initializer(stddev=0.001)", ""}
	return &codegen.TrainIR{
		DataSource:       fmt.Sprintf("mysql://%s", cfg.FormatDSN()),
		Select:           "SELECT deep FROM kaggle_credit_fraud_training_data;",
		ValidationSelect: "SELECT deep FROM kaggle_credit_fraud_training_data;",
		Estimator:        "DNNClassifier",
		Attributes: map[string]interface{}{
			"engine.type":        "yarn",
			"train.max_steps":    1000,
			"model.dnn_hidden_units": []int{10, 20}},
		Features: map[string][]codegen.FeatureColumn{
			"dnn_feature_columns": {
				deepEmbedding}},
		Save: "model_table",
		Label: &codegen.NumericColumn{&codegen.FieldMeta{"c3", codegen.Int, ",", []int{1}, false, nil, 0}}}
	
}
