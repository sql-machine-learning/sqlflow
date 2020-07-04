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
	"fmt"
	"strings"

	"github.com/stretchr/testify/assert"
)

func parserAcceptSemicolon(p Parser) bool {
	switch pr := p.(type) {
	case *javaParser:
		if pr.typ == "odps" {
			return true
		}
		return false
	default:
		return true
	}
}

func commonThirdPartyCases(p Parser, a *assert.Assertions) {
	// one standard SQL statement
	for _, sql := range SelectCases {
		s, idx, err := p.Parse(sql + ";")
		a.NoError(err)
		a.Equal(-1, idx)
		a.Equal(1, len(s))
		if parserAcceptSemicolon(p) {
			a.Equal(sql+`;`, s[0].String)
		} else {
			a.Equal(sql, s[0].String)
		}
	}

	{ // several standard SQL statements with comments
		sqls := strings.Join(SelectCases, `;`) + `;`
		s, idx, err := p.Parse(sqls)
		a.NoError(err)
		a.Equal(-1, idx)
		a.Equal(len(SelectCases), len(s))
		for i := range s {
			if parserAcceptSemicolon(p) {
				a.Equal(SelectCases[i]+`;`, s[i].String)
			} else {
				a.Equal(SelectCases[i], s[i].String)
			}
		}
	}

	// two SQL statements, the first one is extendedSQL
	for _, sql := range SelectCases {
		sqls := fmt.Sprintf(`%s to train;%s;`, sql, sql)
		s, idx, err := p.Parse(sqls)
		a.NoError(err)
		a.Equal(len(sql)+1, idx)
		a.Equal(1, len(s))
		a.Equal(sql+" ", s[0].String)
	}

	// two SQL statements, the second one is extendedSQL
	for _, sql := range SelectCases {
		sqls := fmt.Sprintf(`%s;%s to train;`, sql, sql)
		s, idx, err := p.Parse(sqls)
		a.NoError(err)
		a.Equal(len(sql)+1+len(sql)+1, idx)
		a.Equal(2, len(s))
		if parserAcceptSemicolon(p) {
			a.Equal(sql+`;`, s[0].String)
		} else {
			a.Equal(sql, s[0].String)
		}
		a.Equal(sql+` `, s[1].String)
	}

	// three SQL statements, the second one is extendedSQL
	for _, sql := range SelectCases {
		sqls := fmt.Sprintf(`%s;%s to train;%s;`, sql, sql, sql)
		s, idx, err := p.Parse(sqls)
		a.NoError(err)
		a.Equal(len(sql)+1+len(sql)+1, idx)
		a.Equal(2, len(s))
		if parserAcceptSemicolon(p) {
			a.Equal(sql+`;`, s[0].String)
		} else {
			a.Equal(sql, s[0].String)
		}
		a.Equal(sql+` `, s[1].String)
	}

	{ // two SQL statements, the first standard SQL has an error.
		sql := `select select 1; select 1 to train;`
		s, idx, err := p.Parse(sql)
		a.Equal(0, len(s))
		a.Equal(0, idx)
		a.NoError(err)
	}

	// two SQL statements, the second standard SQL has an error.
	for _, sql := range SelectCases {
		sqls := fmt.Sprintf(`%s to train; select select 1;`, sql)
		s, idx, err := p.Parse(sqls)
		a.NoError(err)
		a.Equal(len(sql)+1, idx)
		a.Equal(1, len(s))
		a.Equal(sql+` `, s[0].String)
	}

	if pr, ok := p.(*javaParser); !ok || pr.typ != "odps" { // non select statement before to train
		sql := `describe table to train;`
		s, idx, err := p.Parse(sql)
		a.Nil(err)
		a.Equal(0, len(s))
		a.Equal(0, idx)
	}

	// show train stmt
	{
		sql := "SHOW TRAIN my_model;"
		stmts, idx, err := p.Parse(sql)
		a.Equal(0, len(stmts))
		a.Equal(0, idx)
		a.Nil(err)
	}
	{
		sql := "select 1; SHOW TRAIN my_model;"
		//                ^ error here
		stmts, idx, err := p.Parse(sql)
		a.Equal(1, len(stmts))
		a.Equal(10, idx)
		a.NoError(err)
	}
	{
		sql := "select 1; -- comment\nSHOW TRAIN my_model;\n--comment"
		//               							^ error here
		stmts, idx, err := p.Parse(sql)
		a.Equal(1, len(stmts))
		a.Equal(21, idx)
		a.Nil(err)
	}
	{
		sql := "select 1; select * from train TO train; SHOW TRAIN my_model;"
		//                                    ^ error here
		stmts, idx, err := p.Parse(sql)
		a.Equal(2, len(stmts))
		a.Equal(30, idx)
		a.Nil(err)
	}
	{
		sql := "SHOW TRAIN my_model; select 1; select * from train TO train;"
		//      ^ error here
		stmts, idx, err := p.Parse(sql)
		a.Equal(0, len(stmts))
		a.Equal(0, idx)
		a.Nil(err)
	}

}
