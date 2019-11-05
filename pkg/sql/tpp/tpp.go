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

package tpp

import "fmt"

func init() {
	tiDBInit()
}

// ParseAndSplit calls driver's parser to parse a SQL program and returns a slice of SQL statements.
//
// It returns <statements, -1, nil> if the parser accepts the SQL program.
//     input:  "select 1; select 1;"
//     output: {"select 1;", "select 1;"}, -1 , nil
// It returns <statements, idx, nil> if the parser accepts part of the SQL program, indicated by idx.
//     input:  "select 1; select 1 to train; select 1"
//     output: {"select 1;", "select 1"}, 19, nil
// It returns <nil, -1, error> if an error is occurred.
func ParseAndSplit(driver, sql string) ([]string, int, error) {
	switch driver {
	case "mysql":
		return tiDBParseAndSplit(sql)
	case "hive":
		return hiveQLParseAndSplit(sql)
	case "calcite":
		return calciteParseAndSplit(sql)
	default:
		return nil, -1, fmt.Errorf("unsupported driver type %s", driver)
	}
}

func hiveQLParseAndSplit(sql string) ([]string, int, error) {
	// TODO(tony): call HiveQL parser via command line
	return nil, -1, fmt.Errorf("hiveQLParseAndSplit not implemented")
}

func calciteParseAndSplit(sql string) ([]string, int, error) {
	// TODO(tony): call Calcite parser via command line
	return nil, -1, fmt.Errorf("calciteParseAndSplit not implemented")
}
