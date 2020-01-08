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

package parser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/pkg/parser/external"
)

func isJavaParser(dbms string) bool {
	return dbms == "hive" || dbms == "calcite"
}

func TestParseWithMySQL(t *testing.T) {
	commonTestCases("mysql", assert.New(t))
}

//func TestParseWithHive(t *testing.T) {
//	commonTestCases("hive", assert.New(t))
//}
//
//func TestParseWithCalcite(t *testing.T) {
//	commonTestCases("calcite", assert.New(t))
//}

func commonTestCases(dbms string, a *assert.Assertions) {
	extendedSQL := `to predict a using b`

	// one standard SQL statement
	for _, sql := range external.SelectCases {
		s, err := Parse(dbms, sql+";")
		a.NoError(err)
		a.Equal(1, len(s))
		a.False(IsExtendedSyntax(s[0]))
		if isJavaParser(dbms) {
			a.Equal(sql, s[0].Standard)
		} else {
			a.Equal(sql+`;`, s[0].Standard)
		}
	}

	{ // several standard SQL statements with comments
		sqls := strings.Join(external.SelectCases, `;`) + `;`
		s, err := Parse(dbms, sqls)
		a.NoError(err)
		a.Equal(len(external.SelectCases), len(s))
		for i := range s {
			a.False(IsExtendedSyntax(s[i]))
			if isJavaParser(dbms) {
				a.Equal(external.SelectCases[i], s[i].Standard)
			} else {
				a.Equal(external.SelectCases[i]+`;`, s[i].Standard)
			}
		}
	}

	// two SQL statements, the first one is extendedSQL
	for _, sql := range external.SelectCases {
		sqls := fmt.Sprintf(`%s %s;%s;`, sql, extendedSQL, sql)
		s, err := Parse(dbms, sqls)
		a.NoError(err)
		a.Equal(2, len(s))

		a.True(IsExtendedSyntax(s[0]))
		a.Equal(sql+` `, s[0].Standard)
		a.Equal(fmt.Sprintf(`%s %s;`, sql, extendedSQL), s[0].Original)

		a.False(IsExtendedSyntax(s[1]))
		if isJavaParser(dbms) {
			a.Equal(sql, s[1].Standard)
		} else {
			a.Equal(sql+`;`, s[1].Standard)
		}
	}

	// two SQL statements, the second one is extendedSQL
	for _, sql := range external.SelectCases {
		sqls := fmt.Sprintf(`%s;%s %s;`, sql, sql, extendedSQL)
		s, err := Parse(dbms, sqls)
		a.NoError(err)
		a.Equal(2, len(s))
		a.False(IsExtendedSyntax(s[0]))
		a.True(IsExtendedSyntax(s[1]))
		if isJavaParser(dbms) {
			a.Equal(sql, s[0].Standard)
		} else {
			a.Equal(sql+`;`, s[0].Standard)
		}
		a.Equal(sql+` `, s[1].Standard)
		a.Equal(fmt.Sprintf(`%s %s;`, sql, extendedSQL), s[1].Original)
	}

	// three SQL statements, the second one is extendedSQL
	for _, sql := range external.SelectCases {
		sqls := fmt.Sprintf(`%s;%s %s;%s;`, sql, sql, extendedSQL, sql)
		s, err := Parse(dbms, sqls)
		a.NoError(err)
		a.Equal(3, len(s))

		// a.False(IsExtendedSyntax(s[0]))
		// a.True(IsExtendedSyntax(s[1]))
		// a.False(IsExtendedSyntax(s[2]))

		a.False(IsExtendedSyntax(s[0]))
		a.True(IsExtendedSyntax(s[1]))
		a.False(IsExtendedSyntax(s[2]))

		if isJavaParser(dbms) {
			a.Equal(sql, s[0].Standard)
			a.Equal(sql, s[2].Standard)
		} else {
			a.Equal(sql+`;`, s[0].Standard)
			a.Equal(sql+`;`, s[2].Standard)
		}

		a.Equal(sql+` `, s[1].Standard)
		a.Equal(fmt.Sprintf(`%s %s;`, sql, extendedSQL), s[1].Original)
	}

	{ // two SQL statements, the first standard SQL has an error.
		sql := `select select 1; select 1 to train;`
		s, err := Parse(dbms, sql)
		a.NotNil(err)
		a.Equal(0, len(s))
	}

	// two SQL statements, the second standard SQL has an error.
	for _, sql := range external.SelectCases {
		sqls := fmt.Sprintf(`%s %s; select select 1;`, sql, extendedSQL)
		s, err := Parse(dbms, sqls)
		a.NotNil(err)
		a.Equal(0, len(s))
	}

	{ // non select statement before to train
		sql := fmt.Sprintf(`describe table %s;`, extendedSQL)
		s, err := Parse(dbms, sql)
		a.NotNil(err)
		a.Equal(0, len(s))
	}
}

func TestParseFirstSQLStatement(t *testing.T) {
	a := assert.New(t)

	{
		pr, idx, e := parseFirstSQLFlowStmt(`to train a with b = c label d into e; select a from b;`)
		a.NotNil(pr)
		a.Equal(len(`to train a with b = c label d into e; `), idx)
		a.NoError(e)
	}

	{
		pr, idx, e := parseFirstSQLFlowStmt(`to train a with b =?? c label d into e ...`)
		a.Nil(pr)
		a.Equal(-1, idx)
		a.Error(e)
	}

	{
		// FIXME(tony): need to update extended syntax parser to make `;` required so that the following case will raise error.
		// ref: https://github.com/sql-machine-learning/sqlflow/pull/1626/files#r363577134
		pr, idx, e := parseFirstSQLFlowStmt(`to train a with b = c label d into e select a from b;`)
		a.NotNil(pr)
		a.Equal(len(`to train a with b = c label d into e `), idx)
		a.NoError(e)
	}
}
