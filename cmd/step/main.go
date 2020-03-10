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
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"time"

	"sqlflow.org/sqlflow/cmd/repl/tablewriter"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql"
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
	fmt.Println("data:text/html,", s)
	fmt.Println()
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
			fmt.Println(s)
		}
	case string:
		fmt.Println(s)
	default:
		return fmt.Errorf("unrecongnized response type: %v", s)
	}
	return nil
}

func runSQLStmt(sqlStmt string, session *pb.Session) {
	startTime := time.Now().UnixNano()
	fmt.Printf("SQLFlow Step Execute:\n%s\n", sqlStmt)

	table, e := tablewriter.Create("protobuf", tablePageSize, os.Stdout)
	if e != nil {
		log.Panicf("create tablewriter failed: %v", e)
	}

	defer func() {
		if e := table.Flush(); e != nil {
			log.Fatal(e)
		}
		fmt.Printf("(%.2f sec)\n", float64(time.Now().UnixNano()-startTime)/1e9)
		fmt.Println()
	}()

	// discard the log output here just because using both log and pipe writer may mix the output
	log.SetOutput(ioutil.Discard)
	stream := sql.RunSQLProgram(sqlStmt, "", session)
	for res := range stream.ReadAll() {
		if e := render(res, table); e != nil {
			log.SetOutput(os.Stdout)
			log.Panic(e)
		}
	}
}

func main() {
	execute := flag.String("execute", "", "execute SQLFlow from command line.  e.g. --execute 'select * from table1'")
	flag.StringVar(execute, "e", "", "execute SQLFlow from command line, short for --execute")
	flag.Parse()

	sess := sql.MakeSessionFromEnv()
	runSQLStmt(*execute, sess)
}
