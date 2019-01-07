package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecutorTrain(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		_, e := Run(testTrainSelectIris, testCfg)
		a.NoError(e)
	})
}

func TestExecutorInfer(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		_, e := Run(testPredictSelectIris, testCfg)
		a.NoError(e)
	})
}

func TestExecutorStandard(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		_, e := Run("show DATABASES;", testCfg)
		a.NoError(e)
	})
}

func TestCreatePredictionTable(t *testing.T) {
	a := assert.New(t)
	trainParsed, e := newParser().Parse(testTrainSelectIris)
	a.NoError(e)
	predParsed, e := newParser().Parse(testPredictSelectIris)
	a.NoError(e)
	a.NoError(createPredictionTable(trainParsed, predParsed, testDB))
}
