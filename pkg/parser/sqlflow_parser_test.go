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

func TestParseWithHive(t *testing.T) {
	commonTestCases("hive", assert.New(t))
}

func TestParseWithCalcite(t *testing.T) {
	commonTestCases("calcite", assert.New(t))
}

func commonTestCases(dbms string, a *assert.Assertions) {
	extendedSQL := `to predict a using b`

	// one standard SQL statement
	for _, sql := range external.SelectCases {
		s, err := Parse(dbms, sql+";")
		a.NoError(err)
		a.Equal(1, len(s))
		a.Nil(s[0].Extended)
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
			a.Nil(s[i].Extended)
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

		a.NotNil(s[0].Extended)
		a.Equal(sql+` `, s[0].Standard)
		a.Equal(fmt.Sprintf(`%s %s;`, sql, extendedSQL), s[0].Original)

		a.Nil(s[1].Extended)
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
		a.Nil(s[0].Extended)
		a.NotNil(s[1].Extended)
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

		a.Nil(s[0].Extended)
		a.NotNil(s[1].Extended)
		a.Nil(s[2].Extended)

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
