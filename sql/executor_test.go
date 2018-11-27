package sql

import (
	"github.com/stretchr/testify/assert"
	"log"
	"testing"
)

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func TestExecutorTrain(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		e := Execute(simpleTrainSelect, testCfg)
		a.NoError(e)
	})

}
