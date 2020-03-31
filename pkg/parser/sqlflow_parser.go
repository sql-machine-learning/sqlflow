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

package parser

import (
	"fmt"
	"strings"

	"sqlflow.org/sqlflow/pkg/parser/external"
)

// SQLFlowStmt represents a parsed SQL statement.  The original
// statement is in Original.  If it is a standard SQL statement,
// Original has the statement as well, and Extended is nil.  Or, if it
// is a statement with SQLFlow syntax extension, Original is the whole
// statement and Extended is the parsed extension.
type SQLFlowStmt struct {
	Original string
	*SQLFlowSelectStmt
	Inputs  []string
	Outputs []string
}

// IsExtendedSyntax returns true if a parsed statement uses any
// SQLFlow syntax extensions.
func (stmt *SQLFlowStmt) IsExtendedSyntax() bool {
	return stmt.SQLFlowSelectStmt != nil
}

// ParseStatement parses a SQL program by calling Parse, and
// asserts that this program contains one and only one statement.
func ParseStatement(dialect, program string) (*SQLFlowStmt, error) {
	stmts, err := Parse(dialect, program)
	if err != nil {
		return nil, err
	}
	if len(stmts) != 1 {
		return nil, fmt.Errorf("expecting one statement, got %d", len(stmts))
	}

	return stmts[0], nil
}

// Parse a SQL program in the given dialect into a list of SQL statements.
func Parse(dialect, program string) ([]*SQLFlowStmt, error) {
	all := []*SQLFlowStmt{}
	for {
		// SELECT ...; SELECT * FROM my_table TO TRAIN ...
		//                                    ^
		//                                    i
		sqls, i, err := thirdPartyParse(dialect, program)
		if err != nil {
			return nil, err
		}
		all = append(all, sqls...)
		if i == -1 { // The third party parser accepts all SQL statements
			return all, nil
		}

		left := all[len(all)-1].Original
		program = program[i:]

		// TO TRAIN dnn LABEL class INTO my_model; SELECT ...
		//                                        ^
		//                                        j
		extended, j, err := parseFirstSQLFlowStmt(program)
		if err != nil {
			return nil, err
		}

		right := program[:j]
		program = program[j:]

		all[len(all)-1].Original = left + right
		all[len(all)-1].SQLFlowSelectStmt = extended
		all[len(all)-1].StandardSelect.origin = left

		if len(strings.TrimSpace(program)) == 0 {
			return all, nil
		}
	}
}

func parseFirstSQLFlowStmt(program string) (*SQLFlowSelectStmt, int, error) {
	pr, idx, err := parseSQLFlowStmt(program)

	if err != nil {
		var e error
		pr, idx, e = parseSQLFlowStmt(program[:idx])
		if e != nil {
			// return the original error since it saw the entire program
			return nil, -1, err
		}
		return pr, idx, nil
	}

	return pr, idx, nil
}

func thirdPartyParse(dialect, program string) ([]*SQLFlowStmt, int, error) {
	p, err := external.NewParser(dialect)
	if err != nil {
		return nil, -1, fmt.Errorf("thirdPartyParse failed: %v", err)
	}
	sqls, i, err := p.Parse(program)
	if err != nil {
		return nil, -1, fmt.Errorf("thirdPartyParse failed: %v", err)
	}
	var spr []*SQLFlowStmt
	for _, sql := range sqls {
		spr = append(spr, &SQLFlowStmt{
			Original:          sql.String,
			Inputs:            sql.Inputs,
			Outputs:           sql.Outputs,
			SQLFlowSelectStmt: nil})
	}
	return spr, i, nil
}
