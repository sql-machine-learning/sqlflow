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

package tidb

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"

	"github.com/pingcap/parser"
	"github.com/pingcap/parser/ast"
	_ "github.com/pingcap/tidb/types/parser_driver" // As required by https://github.com/pingcap/parser/blob/master/parser_example_test.go#L19
)

var (
	// Use the global variable to save the time of creating parser.
	psr *parser.Parser
	re  *regexp.Regexp
	mu  sync.Mutex
)

// Init creates the TiDB parser instance and other resources.
func Init() {
	mu.Lock()
	defer mu.Unlock()

	psr = parser.New()
	re = regexp.MustCompile(`.* near "([^"]+)".*`)
}

// ParseAndSplit calls TiDB's parser to parse a SQL program and returns a slice of SQL statements.
//
// It returns <statements, -1, nil> if TiDB parser accepts the SQL program.
//     input:  "select 1; select 1;"
//     output: {"select 1;", "select 1;"}, -1 , nil
// It returns <statements, idx, nil> if TiDB parser accepts part of the SQL program, indicated by idx.
//     input:  "select 1; select 1 to train; select 1"
//     output: {"select 1;", "select 1"}, 19, nil
// It returns <nil, -1, error> if an error is occurred.
func ParseAndSplit(sql string) ([]string, int, error) {
	if psr == nil || re == nil {
		log.Fatalf("Parser must be called after Init")
	}

	mu.Lock()
	defer mu.Unlock()

	nodes, _, err := psr.Parse(sql, "", "")
	if err != nil {
		matched := re.FindAllStringSubmatch(err.Error(), -1)
		if len(matched) != 1 || len(matched[0]) != 2 {
			return nil, -1, fmt.Errorf(`cannot match parse error "near" in "%q"`, err)
		}
		idx := strings.Index(sql, matched[0][1])

		nodes, _, e := psr.Parse(sql[:idx], "", "")
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
		return sqls, idx, nil
	}

	sqls := make([]string, 0)
	for _, n := range nodes {
		sqls = append(sqls, n.Text())
	}
	return sqls, -1, nil
}
