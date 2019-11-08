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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testStandardExecutiveSQLStatement = `DELETE FROM iris.train WHERE class = 4;`
	testSelectIris                    = `SELECT * FROM iris.train`
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

func TestStandardSQL(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		rd, wr := Pipe()
		go func() {
			defer wr.Close()
			runStandardSQL(wr, testSelectIris, testDB)
		}()
		a.True(goodStream(rd.ReadAll()))
	})
	a.NotPanics(func() {
		if getEnv("SQLFLOW_TEST_DB", "mysql") == "hive" {
			t.Skip("hive: skip DELETE statement")
		}
		rd, wr := Pipe()
		go func() {
			defer wr.Close()
			runStandardSQL(wr, testStandardExecutiveSQLStatement, testDB)
		}()
		a.True(goodStream(rd.ReadAll()))
	})
	a.NotPanics(func() {
		rd, wr := Pipe()
		go func() {
			defer wr.Close()
			runStandardSQL(wr, "SELECT * FROM iris.iris_empty LIMIT 10;", testDB)
		}()
		stat, _ := goodStream(rd.ReadAll())
		a.True(stat)
	})
}

func TestSQLLexerError(t *testing.T) {
	a := assert.New(t)
	stream := RunSQLProgram([]string{"SELECT * FROM ``?[] AS WHERE LIMIT;"}, testDB, "", nil)
	a.False(goodStream(stream.ReadAll()))
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
