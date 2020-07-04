// Copyright 2020 The SQLFlow Authors. All rights reserved.
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

package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/pipe"
	"sqlflow.org/sqlflow/go/test"
)

const (
	testStandardExecutiveSQLStatement = `DELETE FROM iris.train WHERE class = 4;`
	testSelectIris                    = `SELECT * FROM iris.train`
)

func TestNormalStmt(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		rd, wr := pipe.Pipe()
		go func() {
			defer wr.Close()
			e := runNormalStmt(wr, testSelectIris, database.GetTestingDBSingleton())
			a.NoError(e)
		}()
		a.True(test.GoodStream(rd.ReadAll()))
	})
	a.NotPanics(func() {
		if test.GetEnv("SQLFLOW_TEST_DB", "mysql") == "hive" {
			t.Skip("hive: skip DELETE statement")
		}
		rd, wr := pipe.Pipe()
		go func() {
			defer wr.Close()
			e := runNormalStmt(wr, testStandardExecutiveSQLStatement, database.GetTestingDBSingleton())
			a.NoError(e)
		}()
		a.True(test.GoodStream(rd.ReadAll()))
	})
	a.NotPanics(func() {
		rd, wr := pipe.Pipe()
		go func() {
			defer wr.Close()
			e := runNormalStmt(wr, "SELECT * FROM iris.iris_empty LIMIT 10;", database.GetTestingDBSingleton())
			a.NoError(e)
		}()
		stat, _ := test.GoodStream(rd.ReadAll())
		a.True(stat)
	})
}

func TestIsQuery(t *testing.T) {
	a := assert.New(t)
	a.True(isQuery("select * from iris.iris"))
	a.True(isQuery("-- comment some thing\nselect * from iris.iris"))
	a.True(isQuery("show create table iris.iris"))
	a.True(isQuery("show databases"))
	a.True(isQuery("show tables"))
	a.True(isQuery("describe iris.iris"))
	a.True(isQuery("desc iris.iris"))
	a.True(isQuery("explain select 1"))

	a.False(isQuery("select * from iris.iris limit 10 into iris.tmp"))
	a.False(isQuery("insert into iris.iris values ..."))
	a.False(isQuery("delete from iris.iris where ..."))
	a.False(isQuery("update iris.iris where ..."))
	a.False(isQuery("drop table"))
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
