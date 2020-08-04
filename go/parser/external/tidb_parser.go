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

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/pingcap/parser"
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/format"
	_ "github.com/pingcap/tidb/types/parser_driver" // As required by https://github.com/pingcap/parser/blob/master/parser_example_test.go#L19
	"vitess.io/vitess/go/vt/sqlparser"
)

type tidbParser struct {
	psr *parser.Parser
	re  *regexp.Regexp // for parsing error messages from the TiDB parser.
	mu  sync.Mutex
}

// Init creates the TiDB parser instance and other resources.
func newTiDBParser() *tidbParser {
	parser := &tidbParser{
		psr: parser.New(),
		re:  regexp.MustCompile(`.* near "([^"]+)".*`)}
	parser.psr.EnableWindowFunc(true)

	return parser
}

func (p *tidbParser) Dialect() string {
	return "tidb"
}

// splitStatementToPieces split program into single statements
// it do not trim any statement, so join(stmts) == program
func (p *tidbParser) splitStatementToPieces(program string) ([]string, error) {
	// TODO(lhw) MySQL may use delimiter command to specify non-';' separator
	// we should process that case later
	// this func's return do not contain ';'
	stmts, e := sqlparser.SplitStatementToPieces(program)
	if e != nil {
		return nil, e
	}
	if len(stmts) == 0 {
		return stmts, nil
	}
	// add ';' to each stmt
	pos := 0
	for i := 0; i < len(stmts)-1; i++ {
		stmts[i] += ";"
		pos += len(stmts[i])
	}
	stmts[len(stmts)-1] = program[pos:]
	return stmts, nil
}

func (p *tidbParser) getLeadingCommentLen(program string) int {
	lexer := sqlparser.NewStringTokenizer(program)
	lexer.AllowComments = true
	pos := 0
	for {
		tok, _ := lexer.Scan()
		if tok != sqlparser.COMMENT {
			break
		}
		pos = lexer.Position - 1
	}
	for _, r := range program[pos:] {
		if !unicode.IsSpace(r) {
			break
		}
		pos += utf8.RuneLen(r)
	}
	return pos
}

// Parse a SQL program into zero, one, or more statements.  In the
// case of error, it returns the location of the parsing error in
// program and an error message.
func (p *tidbParser) Parse(program string) ([]*Statement, int, error) {
	if p.psr == nil || p.re == nil {
		return nil, -1, fmt.Errorf("parser is not initialized")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	ss, pos, err := p.doParse(program)
	if err != nil || pos < 0 {
		return ss, pos, err
	}
	pos += p.getLeadingCommentLen(program[pos:])
	return ss, pos, err
}

// parseInputsOutputs parse the SELECT ast node and write inputs/outputs table names in retStmt.
func parseInputsOutputs(node ast.StmtNode, retStmt *Statement) {
	restoreFlags := format.RestoreStringSingleQuotes | format.RestoreKeyWordUppercase
	switch node.(type) {
	case *ast.SelectStmt:
		n := node.(*ast.SelectStmt)
		if n.From != nil {
			var sb strings.Builder
			// TODO(typhoonzero): Deal with JOIN clause
			// TODO(typhoonzero): Deal with nested SELECT
			n.From.Restore(format.NewRestoreCtx(restoreFlags, &sb))
			retStmt.Inputs = append(retStmt.Inputs, sb.String())
		}
		// case *ast.UnionStmt: is also query statement, should find way to deal with it
		// deal with insert, update, drop etc.
	case *ast.CreateTableStmt:
		n := node.(*ast.CreateTableStmt)
		retStmt.Outputs = append(retStmt.Outputs, n.Table.Name.String())
		// TODO(typhoonzero): deal with AS SELECT * FROM table, which table is a input
		slctNode, ok := n.Select.(*ast.SelectStmt)
		if ok {
			from := slctNode.From
			if from != nil {
				var sb strings.Builder
				from.Restore(format.NewRestoreCtx(restoreFlags, &sb))
				retStmt.Inputs = append(retStmt.Inputs, sb.String())
			}
		}
	}
}

func (p *tidbParser) doParse(program string) ([]*Statement, int, error) {
	// split program into single statements
	stmts, err := p.splitStatementToPieces(program)
	if err != nil {
		return nil, -1, err
	}
	// error pos or -1 on success
	pos := 0
	retStmts := []*Statement{}
	for _, sql := range stmts {
		nodes, _, err := p.psr.Parse(sql, "", "")
		if err == nil {
			if len(nodes) == 0 { // only comment
				pos += len(sql)
			} else if len(nodes) > 1 {
				return nil, -1, fmt.Errorf("sql statement split failed")
			} else {
				retStmts = append(retStmts, &Statement{String: nodes[0].Text()})
				pos += len(sql)
				parseInputsOutputs(nodes[0], retStmts[len(retStmts)-1])
			}
			continue
		}
		// err occurred
		matched := p.re.FindAllStringSubmatch(err.Error(), -1)
		if len(matched) != 1 || len(matched[0]) != 2 {
			return nil, -1, fmt.Errorf(`cannot match parse error "near" in "%q"`, err)
		}
		idx := strings.Index(sql, matched[0][1])

		// Note(tony): MySQL statements requires adding ";" at
		// the end of the statement.  If we don't add ";",
		// parse("select 1\n").Text() gives "select 1" without
		// the new line character.  This would cause "select
		// 1\nto train" to become "select 1to train" during
		// train SQL saving.
		nodes, _, e := p.psr.Parse(sql[:idx]+";", "", "")
		if e != nil || len(nodes) == 0 {
			// return successfully parsed statements
			return retStmts, pos, nil
		}

		// Make sure the left hand side is a select statement, so that
		// we can try parse the right hand side with the SQLFlow parser
		switch nodes[len(nodes)-1].(type) {
		case *ast.SelectStmt, *ast.UnionStmt:
			pos += idx
			retStmts = append(retStmts, &Statement{String: sql[:idx], IsUnfinishedSelect: true})
		}
		return retStmts, pos, nil
	}
	// program is fully accepted
	return retStmts, -1, nil
}
