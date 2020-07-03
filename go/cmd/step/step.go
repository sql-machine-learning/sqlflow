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
	"log"

	pb "sqlflow.org/sqlflow/go/proto"
	"sqlflow.org/sqlflow/go/sql"
	"sqlflow.org/sqlflow/go/step"
	"sqlflow.org/sqlflow/go/step/tablewriter"
)

const tablePageSize = 1000

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

	return step.RunSQLProgramAndPrintResult(sqlStmt, "", sess, tw, false, false)
}

func main() {
	execute := flag.String("execute", "", "execute SQLFlow from command line.  e.g. --execute 'select * from table1'")
	flag.StringVar(execute, "e", "", "execute SQLFlow from command line, short for --execute")
	flag.Parse()

	if e := run(*execute, sql.MakeSessionFromEnv()); e != nil {
		log.Fatal(e)
	}
}
