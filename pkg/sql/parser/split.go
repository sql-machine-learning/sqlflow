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
	"sqlflow.org/sqlflow/parser/tidb"
	"strings"
)

func init() {
	tidb.Init()
}

func sqlflowParser(sql string) (string, error) {
	// FIXME(tony): only supports "to train" for prototyping.
	// Substitute this function for real SQLFlow parser later.
	extendedSyntax := "to train;"
	if strings.HasPrefix(sql, extendedSyntax) {
		return extendedSyntax, nil
	}
	return "", fmt.Errorf("SQLFlow parser error %v", sql)
}

func split(sql string) ([]string, error) {
	i, err := tidb.Parse(sql)
	if err != nil {
		return nil, err
	}
	if i == -1 { // No error in parsing
		return tidb.Split(sql)
	}

	sqls, err := tidb.Split(sql[:i])
	if err != nil {
		return nil, err
	}

	sql = sql[i:]
	s, err := sqlflowParser(sql)
	if err != nil {
		return nil, err
	}
	sqls[len(sqls)-1] += s

	sql = sql[len(s):]
	nextSqls, err := split(sql)
	if err != nil {
		return nil, err
	}

	return append(sqls, nextSqls...), err
}
