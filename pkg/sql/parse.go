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
	"sqlflow.org/sqlflow/pkg/sql/tpp"
	"strings"
)

type statementParseResult struct {
	standard string
	extended *extendedSelect
}

func extendedSyntaxParse(sql string) (*extendedSelect, int, error) {
	// Note(tony): our parser only supports parsing one statement.
	// So we need to extract the first statement for it.
	s, err := SplitMultipleSQL(sql)
	if err != nil {
		return nil, -1, err
	}

	pr, err := newParser().Parse(s[0])
	if err != nil {
		return nil, -1, err
	}

	return pr, len(s[0]), nil
}

func thirdPartyParse(dbms, sqlProgram string) ([]statementParseResult, int, error) {
	sqls, i, err := tpp.ParseAndSplit(dbms, sqlProgram)
	if err != nil {
		return nil, -1, err
	}
	spr := make([]statementParseResult, 0)
	for _, sql := range sqls {
		spr = append(spr, statementParseResult{standard: sql, extended: nil})
	}
	return spr, i, nil
}

func parse(dbms, sqlProgram string) ([]statementParseResult, error) {
	if len(strings.TrimSpace(sqlProgram)) == 0 {
		return make([]statementParseResult, 0), nil
	}

	sqls, i, err := thirdPartyParse(dbms, sqlProgram)
	if err != nil {
		return nil, err
	}
	if i == -1 { // The third party parser accepts all SQL statements
		return sqls, nil
	}

	sqlProgram = sqlProgram[i:]
	extended, i, err := extendedSyntaxParse(sqlProgram)
	if err != nil {
		return nil, err
	}
	sqls[len(sqls)-1].extended = extended

	sqlProgram = sqlProgram[i:]
	nextSqls, err := parse(dbms, sqlProgram)
	if err != nil {
		return nil, err
	}

	return append(sqls, nextSqls...), err
}
