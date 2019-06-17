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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrainALPSFiller(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	wndStatement := `select dense, deep, wide from kaggle_credit_fraud_training_data 
		TRAIN DNNLinearCombinedClassifier 
		WITH 
			estimator.dnn_hidden_units = [10, 20],
			train_spec.max_steps = 1000
		COLUMN
			DENSE(dense, 5, comma),
			SPARSE(deep, 2000, comma),
			NUMERIC(dense, 5),
			EMBEDDING(CAT_ID(deep, 2000), 8, mean) FOR dnn_feature_columns
		COLUMN
			SPARSE(wide, 1000, comma),
			EMBEDDING(CAT_ID(wide, 1000), 16, mean) FOR linear_feature_columns
		LABEL c3 INTO model_table;`

	r, e := parser.Parse(wndStatement)
	a.NoError(e)

	filler, e := newALPSTrainFiller(r)
	a.NoError(e)

	a.True(filler.IsTraining)
	a.Equal("kaggle_credit_fraud_training_data", filler.TrainInputTable)
	a.Equal("[\"dense\",\"deep\",\"wide\"]", filler.Fields)
	a.True(strings.Contains(filler.X, "SparseColumn(name=\"deep\", shape=[2000], dtype=\"float\", separator=\",\")"))
	a.True(strings.Contains(filler.X, "SparseColumn(name=\"wide\", shape=[1000], dtype=\"float\", separator=\",\")"))
	a.True(strings.Contains(filler.X, "DenseColumn(name=\"dense\", shape=[5], dtype=\"float\", separator=\",\")"))
	a.Equal("DenseColumn(name=\"c3\", shape=[1], dtype=\"int\", separator=\",\")", filler.Y)
	a.True(strings.Contains(filler.EstimatorCreatorCode, "tf.estimator.DNNLinearCombinedClassifier(dnn_hidden_units=[10,20]"))
	a.True(strings.Contains(filler.EstimatorCreatorCode, "linear_feature_columns=[tf.feature_column.embedding_column(tf.feature_column.categorical_column_with_identity(key=\"wide\", num_buckets=1000), dimension=16, combiner=\"mean\")]"))
	a.Equal("1000", filler.TrainSpec["max_steps"])
}
