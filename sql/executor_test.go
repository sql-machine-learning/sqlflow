package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func goodStream(stream chan interface{}) bool {
	for rsp := range stream {
		switch rsp.(type) {
		case error:
			return false
		}
	}
	return true
}

func TestExecutorTrainAndPredict(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		pr, e := newParser().Parse(testTrainSelectIris)
		a.NoError(e)
		stream := runExtendedSQL(testTrainSelectIris, testDB, pr)
		a.True(goodStream(stream.ReadAll()))

		pr, e = newParser().Parse(testPredictSelectIris)
		a.NoError(e)
		stream = runExtendedSQL(testPredictSelectIris, testDB, pr)
		a.True(goodStream(stream.ReadAll()))
	})
}

func TestStandardSQL(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		stream := runStandardSQL(testSelectIris, testDB)
		a.True(goodStream(stream.ReadAll()))
	})
	a.NotPanics(func() {
		stream := runStandardSQL(testStandardExecutiveSQLStatement, testDB)
		a.True(goodStream(stream.ReadAll()))
	})
}

func TestCreatePredictionTable(t *testing.T) {
	a := assert.New(t)
	trainParsed, e := newParser().Parse(testTrainSelectIris)
	a.NoError(e)
	predParsed, e := newParser().Parse(testPredictSelectIris)
	a.NoError(e)
	a.NoError(createPredictionTable(trainParsed, predParsed, testDB.Conn))
}

func TestIsQuery(t *testing.T) {
	a := assert.New(t)
	a.True(isQuery("select * from iris.iris"))
	a.True(isQuery("show create table iris.iris"))
	a.True(isQuery("show databases"))
	a.True(isQuery("show tables"))
	a.True(isQuery("describe iris.iris"))

	a.False(isQuery("select * from iris.iris limit 10 into iris.tmp"))
	a.False(isQuery("insert into iris.iris values ..."))
	a.False(isQuery("delete from iris.iris where ..."))
	a.False(isQuery("update iris.iris where ..."))
	a.False(isQuery("drop table"))
}

func TestLogChanWriter_Write(t *testing.T) {
	a := assert.New(t)
	rd, wr := Pipe()
	go func() {
		defer wr.Close()
		cw := &logChanWriter{wr: wr}
		cw.Write([]byte("hello\n世界"))
		cw.Write([]byte("hello\n世界"))
		cw.Write([]byte("\n"))
		cw.Write([]byte("世界\n世界\n世界\n"))
	}()

	c := rd.ReadAll()

	a.Equal("hello\n", <-c)
	a.Equal("世界hello\n", <-c)
	a.Equal("世界\n", <-c)
	a.Equal("世界\n", <-c)
	a.Equal("世界\n", <-c)
	a.Equal("世界\n", <-c)
	_, more := <-c
	a.False(more)
}
