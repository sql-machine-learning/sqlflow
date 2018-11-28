package sql

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExecutorTrain(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		e := run(simpleTrainSelect, testCfg)
		a.NoError(e)
	})

}
