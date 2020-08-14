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

package external

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExternalParserCommonCasesForMySQL(t *testing.T) {
	a := assert.New(t)
	p, _ := NewParser("mysql")
	commonThirdPartyCases(p, a)
}

func TestExternalParserCommonCasesForTiDB(t *testing.T) {
	a := assert.New(t)
	p, _ := NewParser("tidb")
	commonThirdPartyCases(p, a)
}

func TestTiDBParseAndSplitIdx(t *testing.T) {
	a := assert.New(t)
	var (
		i int
		e error
	)

	p := newTiDBParser()

	_, i, e = p.Parse("SELECTED a FROM t1") // SELECTED => SELECT
	a.Equal(0, i)
	a.NoError(e)

	_, i, e = p.Parse("SELECT * FROM t1 TO TRAIN DNNClassifier")
	a.Equal(17, i)
	a.NoError(e)

	_, i, e = p.Parse("SELECT * FROM t1 TO TO TRAIN DNNClassifier")
	a.Equal(17, i)
	a.NoError(e)

	_, i, e = p.Parse("SELECT * FROM t1 t2 TO TRAIN DNNClassifier") // t2 is an alias of t1
	a.Equal(20, i)
	a.NoError(e)

	_, i, e = p.Parse("SELECT * FROM t1 t2, t3 TO TRAIN DNNClassifier") // t2 is an alias of t1
	a.Equal(24, i)
	a.NoError(e)

	_, i, e = p.Parse("SELECT * FROM t1 t2, t3 t4 TO TRAIN DNNClassifier") // t2 and t4 are aliases.
	a.Equal(27, i)
	a.NoError(e)

	_, i, e = p.Parse("SELECT * FROM (SELECT * FROM t1)")
	a.Equal(-1, i)
	a.Error(e) // TiDB parser and MySQL require an alias name after the nested SELECT.

	_, i, e = p.Parse("SELECT * FROM (SELECT * FROM t1) t2")
	a.Equal(-1, i)
	a.NoError(e)

	_, i, e = p.Parse("SELECT * FROM (SELECT * FROM t1) t2 TO TRAIN DNNClassifier")
	a.Equal(36, i)
	a.NoError(e)
}

func TestSplitSql(t *testing.T) {
	p := newTiDBParser()
	a := assert.New(t)
	{
		ss, e := p.splitStatementToPieces("")
		a.Equal(0, len(ss))
		a.Nil(e)
	}
	{
		ss, e := p.splitStatementToPieces(";")
		a.Equal(1, len(ss))
		a.Equal(";", ss[0])
		a.Nil(e)
	}
	{
		ss, e := p.splitStatementToPieces(";;")
		a.Equal(2, len(ss))
		a.Equal(";", ss[0])
		a.Equal(";", ss[1])
		a.Nil(e)
	}
	{
		ss, e := p.splitStatementToPieces(" ;  ;   ")
		a.Equal(3, len(ss))
		a.Equal(" ;", ss[0])
		a.Equal("  ;", ss[1])
		a.Equal("   ", ss[2])
		a.Nil(e)
	}
	{ // unexpected EOF
		ss, e := p.splitStatementToPieces("\"")
		a.Equal(0, len(ss))
		a.Nil(e)
	}
	{ // ; in comments
		ss, e := p.splitStatementToPieces("-- comment ; \n select 1;")
		a.Equal(1, len(ss))
		a.Nil(e)
	}
	{ // ; in comments
		ss, e := p.splitStatementToPieces("select /* ;;;; */ 1;")
		a.Equal(1, len(ss))
		a.Nil(e)
	}
	{ // ; in comments, on one line
		sql := "--comment 1; select /* ;;;; */ 1;"
		ss, e := p.splitStatementToPieces(sql)
		a.Equal(1, len(ss))
		a.Equal(sql, ss[0])
		a.Nil(e)
	}
	{ // ; in comments
		sql := "--comment 1;\nselect /* ;;;; */ 1; -- comment ; abc  "
		ss, e := p.splitStatementToPieces(sql)
		a.Equal(2, len(ss))
		a.Equal("--comment 1;\nselect /* ;;;; */ 1;", ss[0])
		a.Equal(" -- comment ; abc  ", ss[1])
		a.Nil(e)
		a.Equal(len(strings.Join(ss, "")), len(sql))
	}
	{ // ; in string
		sql := "select * from a where f1 like '%;';select 1"
		ss, e := p.splitStatementToPieces(sql)
		a.Equal(2, len(ss))
		a.Equal("select * from a where f1 like '%;';", ss[0])
		a.Equal("select 1", ss[1])
		a.Nil(e)
		a.Equal(len(strings.Join(ss, "")), len(sql))
	}
	{
		for _, sql := range SelectCases {
			blob := sql + ";" + sql
			ss, e := p.splitStatementToPieces(blob)
			a.Nil(e)
			a.Equal(len(strings.Join(ss, "")), len(blob))
		}
	}
}

func TestGetLeadingCommentLen(t *testing.T) {
	p := newTiDBParser()
	a := assert.New(t)
	a.Equal(0, p.getLeadingCommentLen(""))
	a.Equal(0, p.getLeadingCommentLen("TO train"))
	a.Equal(12, p.getLeadingCommentLen("-- comment \nTO train"))
	a.Equal(13, p.getLeadingCommentLen("/* commnt */ hello"))
	a.Equal(12, p.getLeadingCommentLen("--\n--a\n--abc"))
	a.Equal(13, p.getLeadingCommentLen("--\n--a\n--abc\nSELECT"))
	a.Equal(12, p.getLeadingCommentLen("-- comment \nSELECT\n--comment"))
	a.Equal(27, p.getLeadingCommentLen("/* commnt */\n/* comment */ hello"))
	a.Equal(13, p.getLeadingCommentLen("/* commnt */ hello /* comment */ hello"))
	a.Equal(25, p.getLeadingCommentLen("--comment \n/* comment;*/ SELECT"))
	a.Equal(25, p.getLeadingCommentLen("/* comment;*/--comment \n SELECT"))
	a.Equal(25, p.getLeadingCommentLen("/* comment;*/\n--comment\n SELECT"))
}

func TestTiDBParseDeps(t *testing.T) {
	a := assert.New(t)

	p := newTiDBParser()
	stmts, _, e := p.Parse("SELECT * FROM origial_table;\nCREATE TABLE prepared AS SELECT * FROM original_table;")
	a.NoError(e)
	a.Equal(2, len(stmts))
	a.Equal("origial_table", stmts[0].Inputs[0])
	a.Equal("prepared", stmts[1].Outputs[0])
	a.Equal("original_table", stmts[1].Inputs[0])
}

func TestTiDBParseWindowFunc(t *testing.T) {
	a := assert.New(t)

	p := newTiDBParser()
	stmts, i, e := p.Parse("SELECT LAG(value, 1) OVER (ORDER BY date) AS value_lag_1 FROM t1;")
	a.NoError(e)
	a.Equal(-1, i)
	a.Equal(1, len(stmts))
}
