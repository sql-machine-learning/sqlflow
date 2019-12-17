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

	"sqlflow.org/sqlflow/pkg/parser/external"
)

// SQLFlowStmt represents a parsed SQL statement.  The original
// statement is in Original.  If it is a standard SQL statement,
// Standard has the statement as well, and Extended is nil.  Or, if it
// is a statement with SQLFlow syntax extension, Standard is the
// prefixed SELECT statement, and Extended is the parsed extension.
type SQLFlowStmt struct {
	Original string
	Standard string
	*SQLFlowSelectStmt
}

// IsExtendedSyntax returns true if a parsed statement uses any
// SQLFlow syntax extensions.
func IsExtendedSyntax(stmt *SQLFlowStmt) bool {
	return stmt.SQLFlowSelectStmt != nil
}

// ParseOneStatement parses a SQL program by calling Parse, and
// asserts that this program contains one and only one statement.
func ParseOneStatement(dialect, program string) (*SQLFlowStmt, error) {
	stmts, err := Parse(dialect, program)
	if err != nil {
		return nil, err
	}
	if len(stmts) != 1 {
		return nil, fmt.Errorf("Program not having only one statement")
	}

	return stmts[0], nil
}

// Parse a SQL program in the given dialect into a list of SQL statements.
func Parse(dialect, program string) ([]*SQLFlowStmt, error) {
	if len(strings.TrimSpace(program)) == 0 {
		return nil, nil
	}

	var allStmts []*SQLFlowStmt

	for {
		// thridPartyParse might accept more than one standard
		// SQL statemetns.
		stmts, i, err := thirdPartyParse(dialect, program)
		if err != nil {
			return nil, fmt.Errorf("thirdPartyParse %v", err)
		}
		if i == -1 {
			// thirdPartyParse accepted the whole program.
			allStmts = append(allStmts, stmts...)
			break
		}

		left := stmts[len(stmts)-1].Standard
		program = program[i:]

		extension, j, err := parseSQLFlowStmt(program)
		right := program[:j]
		program = program[j:]

		stmts[len(stmts)-1].Original = left + right
		stmts[len(stmts)-1].SQLFlowSelectStmt = extension

		allStmts = append(allStmts, stmts...)
		if err == nil {
			break // parseSQLFlowStmt accepted all.
		}
	}
	return allStmts, nil
}

func thirdPartyParse(dialect, program string) ([]*SQLFlowStmt, int, error) {
	p := external.NewParser(dialect)
	sqls, i, err := p.Parse(program)
	if err != nil {
		return nil, -1, fmt.Errorf("thirdPartyParse failed: %v", err)
	}
	var spr []*SQLFlowStmt
	for _, sql := range sqls {
		spr = append(spr, &SQLFlowStmt{
			Original:          sql,
			Standard:          sql,
			SQLFlowSelectStmt: nil})
	}
	return spr, i, nil
}

// LegacyParse calls extended_syntax_parser.y with old rules.
// codegen_alps.go depends on this legacy parser, which requires
// extended_syntax_parser.y to parse not only the syntax extension,
// but also the SELECT statement prefix.
func LegacyParse(s string) (r *SQLFlowSelectStmt, idx int, e error) {
	return parseSQLFlowStmt(s)
}
