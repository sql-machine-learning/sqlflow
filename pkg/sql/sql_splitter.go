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
	"fmt"
	"strings"
)

// splitExtendedSQL splits an extended select statement into
// its select clause and the rest. For example,
//
// input:
//   "select ... train ... with ..."
// output:
//   ["select ...", "train ... with ..."].
//
// input:
//   "select ... predict ... using ..."
// output:
//   ["select ...", "predict ... using ..."].
//
// input:
//   "select ..."
// output:
//   ["select ..."]
func splitExtendedSQL(slct string) ([]string, error) {
	l := newLexer(slct)
	var n sqlSymType
	var typ []int
	var pos []int
	for {
		t := l.Lex(&n)
		if t < 0 {
			return []string{}, fmt.Errorf("Lex: Unknown problem %s", slct[0-t:])
		}
		if t == 0 {
			break
		}
		typ = append(typ, t)
		pos = append(pos, l.pos)
	}
	for i := 1; i < len(typ)-2; i++ {
		if (typ[i] == TRAIN && typ[i+1] == IDENT && typ[i+2] == WITH) ||
			(typ[i] == PREDICT && typ[i+1] == IDENT && typ[i+2] == USING) ||
			(typ[i] == PREDICT && typ[i+1] == IDENT && typ[i+2] == WITH) ||
			(typ[i] == EXPLAIN && typ[i+1] == IDENT && typ[i+2] == WITH) ||
			(typ[i] == EXPLAIN && typ[i+1] == IDENT && typ[i+2] == USING) {
			return []string{slct[:pos[i-1]], slct[pos[i-1]:]}, nil
		}
	}

	return []string{slct}, nil
}

// SplitMultipleSQL returns a list of SQL statements if the input statements contains multiple
// SQL statements separated by ;
func SplitMultipleSQL(statements string) ([]string, error) {
	l := newLexer(statements)
	var n sqlSymType
	var sqlList []string
	splitPos := 0
	for {
		t := l.Lex(&n)
		if t < 0 {
			return []string{}, fmt.Errorf("Lex: Unknown problem %s", statements[0-t:])
		}
		if t == 0 {
			if len(sqlList) == 0 {
				// NOTE: this line support executing SQL statement without a trailing ";"
				sqlList = append(sqlList, statements)
			}
			break
		}
		if t == ';' {
			splitted := statements[splitPos:l.pos]
			splitted = strings.TrimSpace(splitted)
			sqlList = append(sqlList, splitted)
			splitPos = l.pos
		}
	}
	return sqlList, nil
}
