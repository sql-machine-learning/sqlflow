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

package step

import (
	"flag"
	"fmt"
	"log"
	"regexp"

	"sqlflow.org/sqlflow/cmd/repl"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql"
	"sqlflow.org/sqlflow/pkg/tablewriter"
)

const tablePageSize = 1000

func isHTMLCode(code string) bool {
	//TODO(yancey1989): support more lines HTML code e.g.
	//<div>
	//  ...
	//</div>
	re := regexp.MustCompile(`<div.*?>.*</div>`)
	return re.MatchString(code)
}

func printAsDataURL(s string) {
	log.Println("data:text/html,", s)
	log.Println()
}

func render(rsp interface{}, table tablewriter.TableWriter) error {
	switch s := rsp.(type) {
	case map[string]interface{}: // table header
		return table.SetHeader(s)
	case []interface{}: // row
		return table.AppendRow(s)
	case error:
		return s
	case sql.EndOfExecution:
	case sql.Figures:
		if isHTMLCode(s.Image) {
			printAsDataURL(s.Image)
		} else {
			log.Println(s)
		}
	case string:
		log.Print(s)
	default:
		return fmt.Errorf("unrecongnized response type: %v", s)
	}
	return nil
}

type logWriter struct{}

func (w *logWriter) Write(b []byte) (int, error) {
	log.Print(string(b))
	return len(b), nil
}

func run(sqlStmt string, sess *pb.Session) error {
	log.Printf("SQLFlow Step Execute:\n%s\n", sqlStmt)
	tw, e := tablewriter.Create("protobuf", tablePageSize, &logWriter{})
	if e != nil {
		log.Fatalf("create tablewriter failed: %v", e)
	}

	return repl.RunSQLProgramAndPrintResult(sqlStmt, "", sess, tw, false)
}

func main() {
	execute := flag.String("execute", "", "execute SQLFlow from command line.  e.g. --execute 'select * from table1'")
	flag.StringVar(execute, "e", "", "execute SQLFlow from command line, short for --execute")
	flag.Parse()

	if e := run(*execute, sql.MakeSessionFromEnv()); e != nil {
		log.Fatal(e)
	}
}
