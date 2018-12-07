package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecutorTrain(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		e := run(testTrainSelectIris, testCfg)
		a.NoError(e)
	})
}

func TestExecutorInfer(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		e := run(testPredictSelectIris, testCfg)
		a.EqualError(e, "infer not implemented")
	})
}
