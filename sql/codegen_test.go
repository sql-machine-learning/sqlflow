package sql

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testSelectIris = `
SELECT *
FROM iris.iris
`
	testTrainSelectIris = testSelectIris + `
TRAIN DNNClassifier
WITH
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO my_dnn_model
;
`
	testPredictSelectIris = testSelectIris + `
predict iris.predict.class
USING my_dnn_model;
`
)

func TestCodeGenTrain(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(testTrainSelectIris)
	a.NoError(e)

	fts, e := verify(r, testDB)
	a.NoError(e)

	a.NoError(genTF(ioutil.Discard, r, fts, testCfg))
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

	a.NoError(genTF(ioutil.Discard, r, fts, testCfg))
}
