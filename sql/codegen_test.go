package sql

import (
	"io/ioutil"
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
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model
;
`
	testPredictSelectIris = `
SELECT *
FROM iris.test
predict iris.predict.class
USING sqlflow_models.my_dnn_model;
`
)

func TestCodeGenTrain(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(testTrainSelectIris)
	a.NoError(e)

	fts, e := verify(r, testDB)
	a.NoError(e)

	a.NoError(genTF(ioutil.Discard, r, fts, testDB))
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

	a.NoError(genTF(ioutil.Discard, r, fts, testDB))
}
