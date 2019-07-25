// Copyright 2019 The SQLFlow Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sql

import (
	"container/list"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func goodStream(stream chan interface{}) (bool, string) {
	lastResp := list.New()
	keepSize := 10

	for rsp := range stream {
		switch rsp.(type) {
		case error:
			var s []string
			for e := lastResp.Front(); e != nil; e = e.Next() {
				s = append(s, e.Value.(string))
			}
			return false, strings.Join(s, "\n")
		}
		lastResp.PushBack(rsp)
		if lastResp.Len() > keepSize {
			e := lastResp.Front()
			lastResp.Remove(e)
		}
	}
	return true, ""
}

func TestTableWithRandomColumn(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		_, e := tableWithRandomColumn(testDB, testGenerateRandomColumnTable)
		if e != errNotSupportYet {
			a.NoError(e)
		}
	})
}

func TestSplitExtendedSQL(t *testing.T) {
	a := assert.New(t)
	s := splitExtendedSQL(`select a train b with c;`)
	a.Equal(2, len(s))
	a.Equal(`select a`, s[0])
	a.Equal(` train b with c;`, s[1])

	s = splitExtendedSQL(`  select a predict b using c;`)
	a.Equal(2, len(s))
	a.Equal(`  select a`, s[0])
	a.Equal(` predict b using c;`, s[1])

	s = splitExtendedSQL(` select a from b;`)
	a.Equal(1, len(s))
	a.Equal(` select a from b;`, s[0])

	s = splitExtendedSQL(`train a with b;`)
	a.Equal(1, len(s))
	a.Equal(`train a with b;`, s[0])
}

func TestExecutorTrainAndPredictDNN(t *testing.T) {
	a := assert.New(t)
	modelDir := ""
	a.NotPanics(func() {
		stream := runExtendedSQL(testTrainSelectIris, testDB, modelDir, nil)
		a.True(goodStream(stream.ReadAll()))

		stream = runExtendedSQL(testPredictSelectIris, testDB, modelDir, nil)
		a.True(goodStream(stream.ReadAll()))
	})
}

func TestExecutorTrainAndPredictDNNLocalFS(t *testing.T) {
	a := assert.New(t)
	modelDir, e := ioutil.TempDir("/tmp", "sqlflow_models")
	a.Nil(e)
	defer os.RemoveAll(modelDir)
	a.NotPanics(func() {
		stream := runExtendedSQL(testTrainSelectIris, testDB, modelDir, nil)
		a.True(goodStream(stream.ReadAll()))

		stream = runExtendedSQL(testPredictSelectIris, testDB, modelDir, nil)
		a.True(goodStream(stream.ReadAll()))
	})
}

func TestExecutorTrainAndPredictionDNNClassifierDENSE(t *testing.T) {
	if getEnv("SQLFLOW_TEST_DB", "mysql") == "hive" {
		t.Skip(fmt.Sprintf("%s: skip Hive test", getEnv("SQLFLOW_TEST_DB", "mysql")))
	}
	a := assert.New(t)
	a.NotPanics(func() {
		stream := Run(`SELECT * FROM iris.train_dense
TRAIN DNNClassifier
WITH
n_classes = 3,
hidden_units = [10, 20],
EPOCHS = 200,
BATCHSIZE = 10
COLUMN NUMERIC(dense, 4)
LABEL class
INTO sqlflow_models.my_dense_dnn_model
;`, testDB, "", nil)
		a.True(goodStream(stream.ReadAll()))
		stream = Run(`SELECT * FROM iris.test_dense
PREDICT iris.predict_dense.class
USING sqlflow_models.my_dense_dnn_model
;`, testDB, "", nil)
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
		if getEnv("SQLFLOW_TEST_DB", "mysql") == "hive" {
			t.Skip("hive: skip DELETE statement")
		}
		stream := runStandardSQL(testStandardExecutiveSQLStatement, testDB)
		a.True(goodStream(stream.ReadAll()))
	})
	a.NotPanics(func() {
		stream := runStandardSQL("SELECT * FROM iris.iris_empty LIMIT 10;", testDB)
		a.True(goodStream(stream.ReadAll()))
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

func TestParseTableColumn(tg *testing.T) {
	a := assert.New(tg)
	t, c, e := parseTableColumn("a.b.c")
	a.NoError(e)
	a.Equal("a.b", t)
	a.Equal("c", c)

	t, c, e = parseTableColumn("a.b")
	a.NoError(e)
	a.Equal("a", t)
	a.Equal("b", c)

	_, _, e = parseTableColumn("a.")
	a.Error(e)
	_, _, e = parseTableColumn("a")
	a.Error(e)
}
