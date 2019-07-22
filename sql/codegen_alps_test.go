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
			model.dnn_hidden_units = [10, 20],
			train.max_steps = 1000
		COLUMN
			DENSE(dense, 5, comma),
			SPARSE(deep, 2000, comma),
			NUMERIC(dense, 5),
			EMBEDDING(CATEGORY_ID(deep, 2000), 8, mean) FOR dnn_feature_columns
		COLUMN
			SPARSE(wide, 1000, comma),
			EMBEDDING(CATEGORY_ID(wide, 1000), 16, mean) FOR linear_feature_columns
		LABEL c3 INTO model_table;`

	r, e := parser.Parse(wndStatement)
	a.NoError(e)

	filler, e := newALPSTrainFiller(r, nil, nil)
	a.NoError(e)

	a.True(filler.IsTraining)
	a.Equal("kaggle_credit_fraud_training_data", filler.TrainInputTable)
	a.True(strings.Contains(filler.X, "SparseColumn(name=\"deep\", shape=[2000], dtype=\"int\")"), filler.X)
	a.True(strings.Contains(filler.X, "SparseColumn(name=\"wide\", shape=[1000], dtype=\"int\")"), filler.X)
	a.True(strings.Contains(filler.X, "DenseColumn(name=\"dense\", shape=[5], dtype=\"float\", separator=\",\")"), filler.X)
	a.Equal("DenseColumn(name=\"c3\", shape=[1], dtype=\"int\", separator=\",\")", filler.Y)
	a.True(strings.Contains(filler.ModelCreatorCode, "tf.estimator.DNNLinearCombinedClassifier(dnn_hidden_units=[10,20]"), filler.ModelCreatorCode)
	a.Equal(1000, filler.TrainClause.MaxSteps)
}
