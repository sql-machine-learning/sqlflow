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

	"sqlflow.org/sqlflow/go/modelzooserver"
)

func main() {
	mysqlDBStr := flag.String("mysql-addr",
		"mysql://root:root@tcp(127.0.0.1:3306)/?",
		"MySQL database connection string for the model zoo server, e.g. mysql://root:root@tcp(127.0.0.1:3306)/?")
	port := flag.Int("port", 50055, "TCP port to listen on.")
	flag.Parse()

	modelzooserver.StartModelZooServer(*port, *mysqlDBStr)
}
