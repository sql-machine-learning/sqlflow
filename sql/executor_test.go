package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TODO(weiguoz): test Run() about stream response

func TestExecutorTrainAndPredict(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		_, e := Run(testTrainSelectIris, testDB, testCfg)
		a.NoError(e)
		_, e = Run(testPredictSelectIris, testDB, testCfg)
		a.NoError(e)
	})
}

func TestExecutorStandard(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		_, e := Run(testSelectIris, testDB, testCfg)
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

func TestLogChanWriter_Write(t *testing.T) {
	a := assert.New(t)

	c := make(chan Response)

	go func() {
		defer close(c)
		cw := &logChanWriter{c: c}
		cw.Write([]byte("hello\n世界"))
		cw.Write([]byte("hello\n世界"))
		cw.Write([]byte("\n"))
		cw.Write([]byte("世界\n世界\n世界\n"))
	}()

	a.Equal("hello\n", (<-c).data)
	a.Equal("世界hello\n", (<-c).data)
	a.Equal("世界\n", (<-c).data)
	a.Equal("世界\n", (<-c).data)
	a.Equal("世界\n", (<-c).data)
	a.Equal("世界\n", (<-c).data)
	_, more := <-c
	a.False(more)
}
