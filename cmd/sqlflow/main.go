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

package main

import (
	"flag"
	"fmt"
	"io/ioutil"

	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/log"
	"sqlflow.org/sqlflow/pkg/parser"
	"sqlflow.org/sqlflow/pkg/sql"
	"sqlflow.org/sqlflow/pkg/workflow"
)

func compile(cgName, sqlProgram, datasource string, logger *log.Logger) (string, error) {
	driverName, _, err := database.ParseURL(datasource)
	if err != nil {
		return "", err
	}
	stmts, err := parser.Parse(driverName, sqlProgram)
	if err != nil {
		return "", err
	}
	switch cgName {
	case "couler":
		spIRs, err := sql.ResolveSQLProgram(stmts, logger)
		if err != nil {
			return "", err
		}
		sess := sql.MakeSessionFromEnv()
		sess.DbConnStr = datasource
		codegen, _, e := workflow.New(cgName)
		if e != nil {
			return "", e
		}
		return codegen.GenCode(spIRs, sess)
	default:
		// TODO(yancey1989): support other codegen, e.g, tensorflow, xgboost.
		return "", fmt.Errorf("sqlflow compiler has not support codegen: %s", cgName)
	}
}

func main() {
	ds := flag.String("datasource", "", "database connect string")
	logPath := flag.String("log", "", "path/to/log, e.g.: /var/log/sqlflow.log")
	cgName := flag.String("codegen", "", "SQLFlow compile the input SQL program into Python program using the specified code generator.")
	flag.StringVar(cgName, "x", "", "short for --codegen")
	sqlFileName := flag.String("file", "", "execute SQLFlow from file.  e.g. --file '~/iris_dnn.sql'")
	flag.StringVar(sqlFileName, "f", "", "short for --file")
	flag.Parse()

	log.SetOutput(*logPath)
	logger := log.GetDefaultLogger()

	sqlProgram, e := ioutil.ReadFile(*sqlFileName)
	if e != nil {
		logger.Fatalf("read file failed, %v", e)
	}

	code, e := compile(*cgName, string(sqlProgram), *ds, logger)
	if e != nil {
		logger.Fatalf("compile failed, %v", e)
	}
	fmt.Println(code)
}
