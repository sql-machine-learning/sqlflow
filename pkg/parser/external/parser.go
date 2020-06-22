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
	Parse(program string) ([]*Statement, int, error)
}

// Statement a parsed SQL statement string and it's input tables and output tables.
type Statement struct {
	String             string
	Inputs             []string
	Outputs            []string
	IsUnfinishedSelect bool
}

// NewParser instantiates a parser.
func NewParser(dialect string) (Parser, error) {
	switch dialect {
	case "mysql", "tidb":
		return newTiDBParser(), nil
	case "hive":
		return newJavaParser("hive"), nil
	case "calcite":
		return newJavaParser("calcite"), nil
	case "maxcompute", "alisa":
		// maxcompute is PHONY parser, java will
		// chose odps or calcite according to which
		// exists in classpath
		return newJavaParser("maxcompute"), nil
	default:
		return nil, fmt.Errorf("unrecognized dialect %s", dialect)
	}
}
