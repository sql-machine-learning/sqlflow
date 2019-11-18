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

import (
	"encoding/json"
	"fmt"
	"github.com/pingcap/parser"
	"github.com/pingcap/parser/ast"
	_ "github.com/pingcap/tidb/types/parser_driver" // As required by https://github.com/pingcap/parser/blob/master/parser_example_test.go#L19
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var (
	// Use the global variable to save the time of creating parser.
	psr *parser.Parser
	re  *regexp.Regexp
	mu  sync.Mutex
)

// Init creates the TiDB parser instance and other resources.
func tiDBInit() {
	mu.Lock()
	defer mu.Unlock()

	psr = parser.New()
	re = regexp.MustCompile(`.* near "([^"]+)".*`)
}

func tiDBParseAndSplit(sql string) ([]string, int, error) {
	if psr == nil || re == nil {
		return nil, -1, fmt.Errorf("parser is not initialized")
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

type parseResult struct {
	Statements []string `json:"statements"`
	Position   int      `json:"position"`
	Error      string   `json:"error"`
}

func javaParseAndSplit(typ, sql string) ([]string, int, error) {
	// cwd is used to store train scripts and save output models.
	cwd, err := ioutil.TempDir("/tmp", "sqlflow")
	if err != nil {
		return nil, -1, err
	}
	defer os.RemoveAll(cwd)

	inputFile := filepath.Join(cwd, "input.sql")
	outputFile := filepath.Join(cwd, "output.json")
	if err := ioutil.WriteFile(inputFile, []byte(sql), 755); err != nil {
		return nil, -1, err
	}

	cmd := exec.Command("java",
		"-cp", "/opt/sqlflow/parser/parser-1.0-SNAPSHOT-jar-with-dependencies.jar",
		"org.sqlflow.parser.ParserAdaptorCmd",
		"-p", typ,
		"-i", inputFile,
		"-o", outputFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Println(string(output))
		return nil, -1, fmt.Errorf("%s %v", output, err)
	}

	output, err := ioutil.ReadFile(outputFile)
	if err != nil {
		return nil, -1, err
	}

	var pr parseResult
	if err = json.Unmarshal(output, &pr); err != nil {
		return nil, -1, err
	}

	if pr.Error != "" {
		return nil, -1, fmt.Errorf(pr.Error)
	}

	return pr.Statements, pr.Position, nil
}
