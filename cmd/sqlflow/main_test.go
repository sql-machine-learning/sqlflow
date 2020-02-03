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
	"testing"

	"github.com/stretchr/testify/assert"
)

var testSQLProgram = `
SELECT * FROM iris.train LIMIT 10;

SELECT *
FROM iris.train
TO TRAIN DNNClassifier
WITH
	model.n_classes = 3,
	model.hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO my_model;

SELECT *
FROM iris.test
TO PREDICT iris.predict.class
USING my_model;
`

func TestWorkflow(t *testing.T) {
	a := assert.New(t)
	datasource := "mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0"
	code, e := compile("couler", testSQLProgram, datasource)
	a.NoError(e)
	expectedCodeSnippet := `couler.run_container(command='''repl -e "%s" --datasource="%s"''' % (train_sql, datasource), image="sqlflow/sqlflow", env=step_envs)`
	a.Contains(code, expectedCodeSnippet)
}
