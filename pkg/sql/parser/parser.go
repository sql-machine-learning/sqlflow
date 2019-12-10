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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

type parseResult struct {
	Statements []string `json:"statements"`
	Position   int      `json:"position"`
	Error      string   `json:"error"`
}

func javaParseAndSplit(typ, program string) ([]string, int, error) {
	// cwd is used to store train scripts and save output models.
	cwd, err := ioutil.TempDir("/tmp", "sqlflow")
	if err != nil {
		return nil, -1, err
	}
	defer os.RemoveAll(cwd)

	inputFile := filepath.Join(cwd, "input.sql")
	outputFile := filepath.Join(cwd, "output.json")
	if err := ioutil.WriteFile(inputFile, []byte(program), 0755); err != nil {
		return nil, -1, err
	}

	// TODO(yi): It is very expensive to start a Java process.  It
	// slows down SQLFlow server's QPS if for every parsing
	// operation, we'd have to start a Java process.
	cmd := exec.Command("java",
		"-cp", "/opt/sqlflow/parser/parser-1.0-SNAPSHOT-jar-with-dependencies.jar",
		"org.sqlflow.parser.ParserAdaptorCmd",
		"-p", typ,
		"-i", inputFile,
		"-o", outputFile)
	if output, err := cmd.CombinedOutput(); err != nil {
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
