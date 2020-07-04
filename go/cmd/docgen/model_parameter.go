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
	"fmt"
	"regexp"
	"strings"

	"sqlflow.org/sqlflow/go/codegen/tensorflow"
	"sqlflow.org/sqlflow/go/codegen/xgboost"
)

func main() {
	fmt.Print(`# Model Parameter Document

SQLFlow connects a SQL engine (e.g., MySQL, Hive, or MaxCompute) and TensorFlow and other machine learning toolkits by extending the SQL syntax. The extended SQL syntax contains the WITH clause where a user specifies the parameters of his/her ML jobs. This documentation lists all parameters supported by SQLFlow.

`)

	docGenFunc := []func() string{
		xgboost.DocGenInMarkdown,
		tensorflow.DocGenInMarkdown,
	}

	section := regexp.MustCompile(`^#{1,5} `)
	for _, f := range docGenFunc {
		lines := strings.Split(f(), "\n")
		for i := range lines {
			// convert title -> section, section -> subsection
			if section.MatchString(lines[i]) {
				lines[i] = "#" + lines[i]
			}
			fmt.Println(lines[i])
		}
	}
}
