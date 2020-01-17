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

	"github.com/pingcap/parser"
	"github.com/pingcap/parser/ast"
	_ "github.com/pingcap/tidb/types/parser_driver" // As required by https://github.com/pingcap/parser/blob/master/parser_example_test.go#L19
)

type tidbParser struct {
	psr *parser.Parser
	re  *regexp.Regexp // for parsing error messages from the TiDB parser.
	mu  sync.Mutex
}

// Init creates the TiDB parser instance and other resources.
func newTiDBParser() *tidbParser {
	return &tidbParser{
		psr: parser.New(),
		re:  regexp.MustCompile(`.* near "([^"]+)".*`)}
}

func (p *tidbParser) Dialect() string {
	return "tidb"
}

// Parse a SQL program into zero, one, or more statements.  In the
// case of error, it returns the location of the parsing error in
// program and an error message.
func (p *tidbParser) Parse(program string) ([]string, int, error) {
	if p.psr == nil || p.re == nil {
		return nil, -1, fmt.Errorf("parser is not initialized")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	nodes, _, err := p.psr.Parse(program, "", "")
	if err != nil {
		matched := p.re.FindAllStringSubmatch(err.Error(), -1)
		if len(matched) != 1 || len(matched[0]) != 2 {
			return nil, -1, fmt.Errorf(`cannot match parse error "near" in "%q"`, err)
		}
		idx := strings.Index(program, matched[0][1])

		// Note(tony): MySQL statements requires adding ";" at
		// the end of the statement.  If we don't add ";",
		// parse("select 1\n").Text() gives "select 1" without
		// the new line character.  This would cause "select
		// 1\nto train" to become "select 1to train" during
		// train SQL saving.
		nodes, _, e := p.psr.Parse(program[:idx]+";", "", "")
		if e != nil || len(nodes) == 0 {
			// return the original parsing error
			return nil, -1, err
		}

		// Make sure the left hand side is a select statement, so that
		// we can try parse the right hand side with the SQLFlow parser
		if _, ok := nodes[len(nodes)-1].(*ast.SelectStmt); !ok {
			// return the original parsing error
			return nil, -1, err
		}

		sqls := make([]string, 0)
		for _, n := range nodes {
			sqls = append(sqls, n.Text())
		}

		// Note(tony): remove the last ";" since feature derivation will append "limit 1000" at the end of the statement
		if sql := sqls[len(sqls)-1]; sql[len(sql)-1] == ';' {
			sqls[len(sqls)-1] = sql[:len(sql)-1]
		}

		return sqls, idx, nil
	}

	sqls := make([]string, 0)
	for _, n := range nodes {
		sqls = append(sqls, n.Text())
	}
	return sqls, -1, nil
}
