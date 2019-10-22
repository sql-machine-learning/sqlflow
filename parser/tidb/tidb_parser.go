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

// Parse calls TiDB's parser to parse a statement sql.  It returns
// <-1,nil> if TiDB parser accepts the statement, or <pos,nil> if TiDB
// doesn't accept but returns a `near "..."` in the error message, or
// <-1,err> if the error messages don't contain near.
func Parse(sql string) (idx int, err error) {
	if psr == nil || re == nil {
		log.Fatalf("Parser must be called after Init")
	}

	mu.Lock()
	defer mu.Unlock()

	if _, _, err = psr.Parse(sql, "", ""); err != nil {
		matched := re.FindAllStringSubmatch(err.Error(), -1)
		if len(matched) != 1 || len(matched[0]) != 2 {
			return -1, fmt.Errorf("cannot match near in %q", err)
		}
		idx = strings.Index(sql, matched[0][1])

		if _, _, e := psr.Parse(sql[:idx], "", ""); e != nil {
			return idx, fmt.Errorf("parsing left hand side \"%s\" failed: %v", sql[:idx], e)
		}
		return idx, nil
	}
	return -1, nil
}

// Split splits a SQL program into a slice of SQL statements
func Split(sql string) ([]string, error) {
	if psr == nil || re == nil {
		log.Fatalf("Parser must be called after Init")
	}

	mu.Lock()
	defer mu.Unlock()

	nodes, _, err := psr.Parse(sql, "", "")
	if err != nil {
		return nil, err
	}

	sqls := make([]string, 0)
	for _, n := range nodes {
		sqls = append(sqls, n.Text())
	}
	return sqls, nil
}
