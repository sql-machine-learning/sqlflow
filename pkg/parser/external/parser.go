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

import "fmt"

// Parser abstract a parser of a SQL engine, for example, Hive, MySQL,
// TiDB, MaxCompute.
type Parser interface {
	Parse(program string) ([]string, []*InputOutputTables, int, error)
}

// InputOutputTables record tables that a SQL statement reads and writes.
type InputOutputTables struct {
	InputTables  []string
	OutputTables []string
}

// NewParser instantiates a parser.
func NewParser(dialect string) (Parser, error) {
	switch dialect {
	case "mysql", "tidb":
		return newTiDBParser(), nil
	case "hive":
		return newJavaParser("hive"), nil
	case "calcite", "maxcompute":
		return newJavaParser("calcite"), nil
	case "alisa":
		return newJavaParser("odps"), nil
	default:
		return nil, fmt.Errorf("unrecognized dialect %s", dialect)
	}
}
