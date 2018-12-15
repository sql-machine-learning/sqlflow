package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecutorTrain(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		e := Run(testTrainSelectIris, testCfg)
		a.NoError(e)
	})
}

func TestExecutorInfer(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		e := Run(testPredictSelectIris, testCfg)
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
