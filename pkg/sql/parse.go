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
	"sqlflow.org/sqlflow/pkg/sql/tpp"
	"strings"
)

// FIXME(tony): only supports "to train" for prototyping.
// Substitute this function for real SQLFlow parser later.
func extendedSyntaxParse(sql string) (string, error) {
	extendedSyntax := "to train;"
	if strings.HasPrefix(sql, extendedSyntax) {
		return extendedSyntax, nil
	}
	return "", fmt.Errorf("SQLFlow parser error %v", sql)
}

// FIXME(tony): change the return type from []string to []parsedResult
func parse(dbms, sqlProgram string) ([]string, error) {
	if len(sqlProgram) == 0 {
		return make([]string, 0), nil
	}

	sqls, i, err := tpp.ParseAndSplit(dbms, sqlProgram)
	if err != nil {
		return nil, err
	}
	if i == -1 { // The third party parser accepts all SQL statements
		return sqls, nil
	}

	sqlProgram = sqlProgram[i:]
	s, err := extendedSyntaxParse(sqlProgram)
	if err != nil {
		return nil, err
	}
	sqls[len(sqls)-1] += s

	sqlProgram = sqlProgram[len(s):]
	nextSqls, err := parse(dbms, sqlProgram)
	if err != nil {
		return nil, err
	}

	return append(sqls, nextSqls...), err
}
