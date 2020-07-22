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

	"sqlflow.org/sqlflow/go/parser/external"
)

// SQLFlowStmt represents a parsed SQL statement.  The original
// statement is in Original.  If it is a standard SQL statement,
// Original has the statement as well, and Extended is nil.  Or, if it
// is a statement with SQLFlow syntax extension, Original is the whole
// statement and Extended is the parsed extension.
type SQLFlowStmt struct {
	IsUnfinishedSelect bool
	Original           string
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
// Notice: If program contains more than one statement,
// we should call sql.RewriteSQLWithHints
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
	//all := []*SQLFlowStmt{{Original: `SHOW create table sqlflow_models.my_dnn_model;`}}
	all := []*SQLFlowStmt{}
	for {
		// SELECT ...; SELECT * FROM my_table TO TRAIN ...
		//                                    ^
		//                                    i
		// or
		// SELECT ...; SHOW TRAIN my_model;
		//            ^
		//            i
		sqls, i, err := thirdPartyParse(dialect, program)

		if err != nil {
			return nil, err
		}
		all = append(all, sqls...)
		if i < 0 {
			return all, nil
		}
		program = program[i:]
		extended, j, err := parseFirstSQLFlowStmt(program)
		if err != nil {
			return nil, err
		}
		// SELECT ... .TO ...
		if len(sqls) > 0 && sqls[len(sqls)-1].IsUnfinishedSelect {
			if extended.ShowTrain {
				return nil, fmt.Errorf("select should followed by 'to train/predict/explain'")
			}
			left := all[len(all)-1].Original
			right := program[:j]
			all[len(all)-1].Original = left + right
			all[len(all)-1].SQLFlowSelectStmt = extended
			all[len(all)-1].StandardSelect.origin = left
			program = program[j:]
		} else {
			// Purely extended sql stmt
			if !extended.ShowTrain {
				return nil, fmt.Errorf("invalid 'to train/predict/explain' with no 'select'")
			}
			sql := &SQLFlowStmt{Original: program[:j], SQLFlowSelectStmt: extended}
			all = append(all, sql)
			program = program[j:]
		}
		if len(strings.TrimSpace(program)) == 0 {
			return all, nil
		}
	}
}

func parseFirstSQLFlowStmt(program string) (*SQLFlowSelectStmt, int, error) {
	// extendedSyntaxDebug = 5
	// extendedSyntaxErrorVerbose = true
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
			Original:           sql.String,
			Inputs:             sql.Inputs,
			Outputs:            sql.Outputs,
			SQLFlowSelectStmt:  nil,
			IsUnfinishedSelect: sql.IsUnfinishedSelect})
	}
	return spr, i, nil
}

// findMatchedQuotationMarks finds the matched "" and '' indices in the
// SQL statement.
func findMatchedQuotationMarks(sql string) ([]int, []int, error) {
	// check whether the quotation mark " or ' is in the form of \" or \'
	isEscapeRune := func(idx int) bool {
		cnt := 0
		for idx--; idx >= 0; idx-- {
			if sql[idx] != '\\' {
				break
			}
			cnt++
		}
		return cnt%2 == 1
	}

	leftQuotationIndex := make([]int, 0)
	rightQuotationIndex := make([]int, 0)
	offset := 0
	for {
		leftIdx := strings.IndexAny(sql[offset:], `"'`)
		if leftIdx < 0 {
			break
		}

		target := rune(sql[offset+leftIdx])
		offset += leftIdx + 1
		for {
			rightIdx := strings.IndexRune(sql[offset:], target)
			if rightIdx < 0 {
				return nil, nil, fmt.Errorf("unmatched quotation marks")
			}

			offset += rightIdx + 1
			if !isEscapeRune(offset - 1) {
				leftQuotationIndex = append(leftQuotationIndex, leftIdx)
				rightQuotationIndex = append(rightQuotationIndex, offset-1)
				break
			}
		}
	}
	return leftQuotationIndex, rightQuotationIndex, nil
}

func removeSingleLineComment(sql string) (string, error) {
	leftQuotationIndices, rightQuotationIndices, err := findMatchedQuotationMarks(sql)
	if err != nil {
		return "", err
	}

	findRightQuotationIndex := func(idx int) int {
		for i := range leftQuotationIndices {
			if idx >= leftQuotationIndices[i] && idx <= rightQuotationIndices[i] {
				return rightQuotationIndices[i]
			}
		}
		return -1
	}

	resultSQL := ""
	offset := 0
	for {
		leftIdx := strings.Index(sql[offset:], "--")
		if leftIdx < 0 {
			resultSQL += sql[offset:]
			break
		}

		rightQuotationIdx := findRightQuotationIndex(offset + leftIdx)
		if rightQuotationIdx >= 0 {
			resultSQL += sql[offset : rightQuotationIdx+1]
			offset = rightQuotationIdx + 1
			continue
		}

		resultSQL += sql[offset : offset+leftIdx]
		rightIdx := strings.Index(sql[offset+leftIdx+2:], "\n")
		if rightIdx < 0 {
			break
		}
		offset += leftIdx + rightIdx + 2
	}
	return resultSQL, nil
}

func removeMultipleLineComment(sql string) (string, error) {
	leftQuotationIndices, rightQuotationIndices, err := findMatchedQuotationMarks(sql)
	if err != nil {
		return "", err
	}

	findRightQuotationIndex := func(idx int) int {
		for i := range leftQuotationIndices {
			if idx >= leftQuotationIndices[i] && idx <= rightQuotationIndices[i] {
				return rightQuotationIndices[i]
			}
		}
		return -1
	}

	resultSQL := ""
	offset := 0
	for {
		leftIdx := strings.Index(sql[offset:], "/*")
		if leftIdx < 0 {
			resultSQL += sql[offset:]
			break
		}

		rightQuotationIdx := findRightQuotationIndex(offset + leftIdx)
		if rightQuotationIdx >= 0 {
			resultSQL += sql[offset : rightQuotationIdx+1]
			offset = rightQuotationIdx + 1
			continue
		}

		resultSQL += sql[offset : offset+leftIdx]
		offset += leftIdx + 2
		rightIdx := strings.Index(sql[offset:], "*/")
		if rightIdx < 0 {
			return "", fmt.Errorf("unmatched comment /*...*/")
		}

		resultSQL += " "
		offset += rightIdx + 2
	}
	return resultSQL, nil
}

// RemoveCommentInSQLStatement removes the comments in the
// SQL statement, including single line comment (--) and multiple
// line comment (/*...*/)
func RemoveCommentInSQLStatement(sql string) (string, error) {
	sql, err := removeMultipleLineComment(sql)
	if err != nil {
		return "", err
	}

	sql, err = removeSingleLineComment(sql)
	if err != nil {
		return "", err
	}
	return sql, nil
}
