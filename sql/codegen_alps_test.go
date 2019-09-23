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

package sql

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	pb "sqlflow.org/sqlflow/server/proto"
	"github.com/stretchr/testify/assert"
)

func TestTrainALPSFiller(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	wndStatement := `SELECT dense, deep, wide FROM kaggle_credit_fraud_training_data 
		TRAIN DNNLinearCombinedClassifier 
		WITH 
			model.dnn_hidden_units = [10, 20],
			train.max_steps = 1000,
			engine.type = "yarn"
		COLUMN
			DENSE(dense, 5, comma),
			SPARSE(deep, 2000, comma),
			NUMERIC(dense, 5),
			EMBEDDING(CATEGORY_ID(deep, 2000), 8, mean) FOR dnn_feature_columns
		COLUMN
			SPARSE(wide, 1000, comma),
			EMBEDDING(CATEGORY_ID(wide, 1000), 16, mean) FOR linear_feature_columns
		LABEL c3
		INTO model_table;`

	r, e := parser.Parse(wndStatement)
	a.NoError(e)
	session := &pb.Session{UserId: "sqlflow_user"}
	filler, e := newALPSTrainFiller(r, nil, session, nil)
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
}

func TestTrainALPSEmbeddingInitializer(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	wndStatement := `SELECT deep FROM kaggle_credit_fraud_training_data 
		TRAIN DNNClassifier 
		WITH 
			model.dnn_hidden_units = [10, 20],
			train.max_steps = 1000,
			engine.type = "yarn"
		COLUMN
			SPARSE(deep, 2000, comma),
			EMBEDDING(CATEGORY_ID(deep, 2000), 8, "sum", "tf.random_normal_initializer(stddev=0.001)") FOR dnn_feature_columns
		LABEL class
		INTO model_table;`

	r, e := parser.Parse(wndStatement)
	a.NoError(e)
	session := &pb.Session{UserId: "sqlflow_user"}
	filler, e := newALPSTrainFiller(r, nil, session, nil)
	a.NoError(e)
	fmt.Println(filler.FeatureColumnCode)
	a.True(strings.Contains(filler.FeatureColumnCode, "tf.feature_column.embedding_column(tf.feature_column.categorical_column_with_identity(key=\"deep_0\", num_buckets=2000), dimension=8, combiner=\"sum\", initializer=tf.random_normal_initializer(stddev=0.001))"))
}

func TestPredALPSFiller(t *testing.T) {
	a := assert.New(t)
	parser := newParser()
	os.Setenv("OSS_KEY", "sqlflow_key")
	os.Setenv("OSS_ID", "sqlflow_id")
	os.Setenv("OSS_ENDPOINT", "http://sqlflow-oss-endpoint")
	predStatement := `SELECT predict_fun(concat(",", col_1, col_2)) AS (info, score) FROM db.table
		PREDICT db.predict_result
		USING sqlflow_model;`

	r, e := parser.Parse(predStatement)
	session := &pb.Session{UserId: "sqlflow_user"}
	filler, e := newALPSPredictFiller(r, session)
	a.NoError(e)

	a.False(filler.IsTraining)
	a.Equal(filler.PredictInputTable, "db.table")
	a.Equal(filler.PredictOutputTable, "db.predict_result")
	a.Equal(filler.PredictUDF, `predict_fun(concat(",", col_1, col_2)) AS (info, score)`)
	a.Equal(filler.ModelDir, "oss://cmps-model/sqlflow/sqlflow_user/sqlflow_model.tar.gz")
	a.Equal(filler.PredictInputModel, "sqlflow_model")
	a.Equal(filler.UserID, "sqlflow_user")
	a.Equal(filler.OSSID, "sqlflow_id")
	a.Equal(filler.OSSKey, "sqlflow_key")

	var program bytes.Buffer
	e = alpsPredTemplate.Execute(&program, filler)
	a.NoError(e)

	arr := strings.Split(program.String(), ";")
	udfSQL := strings.Trim(arr[len(arr)-2], "\n")
	a.Equal(udfSQL,
		`CREATE TABLE IF NOT EXISTS db.predict_result AS `+
			`SELECT predict_fun(concat(",", col_1, col_2)) AS (info, score) FROM db.table`)
}
