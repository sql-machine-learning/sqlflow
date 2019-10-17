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
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testStandardExecutiveSQLStatement = `DELETE FROM iris.train WHERE class = 4;`
	testSelectIris                    = `
SELECT *
FROM iris.train
`
	testTrainSelectIris = testSelectIris + `
TRAIN DNNClassifier
WITH
  model.n_classes = 3,
  model.hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;
`
	testPredictSelectIris = `
SELECT *
FROM iris.test
predict iris.predict.class
USING sqlflow_models.my_dnn_model;
`
	testClusteringTrain = testSelectIris + `
TRAIN sqlflow_models.DeepEmbeddingClusterModel
WITH
  model.pretrain_dims = [10,10],
  model.n_clusters = 3,
  model.pretrain_lr = 0.001,
  train.batch_size = 1
COLUMN sepal_length, sepal_width, petal_length, petal_width
INTO sqlflow_models.my_clustering_model;
`
	testClusteringPredict = `
SELECT *
FROM iris.test
PREDICT iris.predict.class
USING sqlflow_models.my_clustering_model;
`
)

func TestCodeGenTrain(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(testTrainSelectIris)
	a.NoError(e)

	fts, e := verify(r, testDB)
	a.NoError(e)

	a.NoError(genTF(ioutil.Discard, r, nil, fts, testDB))
}

func TestCodeGenPredict(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(testTrainSelectIris)
	a.NoError(e)
	tc := r.trainClause

	r, e = newParser().Parse(testPredictSelectIris)
	a.NoError(e)
	r.trainClause = tc

	fts, e := verify(r, testDB)
	a.NoError(e)

	a.NoError(genTF(ioutil.Discard, r, nil, fts, testDB))
}

func TestLabelAsStringType(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(`SELECT customerID, gender FROM churn.train
TRAIN DNNClassifier
WITH
	model.n_classes = 3,
	model.hidden_units = [10, 20]
COLUMN customerID
LABEL gender
INTO sqlflow_models.my_dnn_model;`)
	a.NoError(e)

	fts, e := verify(r, testDB)
	a.NoError(e)
	e = genTF(ioutil.Discard, r, nil, fts, testDB)
	a.NotNil(e)
	a.True(strings.HasPrefix(e.Error(), "unsupported label data type:"))
}
