package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecutorTrain(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		e := run(testTrainSelectChurn, testCfg)
		a.NoError(e)
	})

}
