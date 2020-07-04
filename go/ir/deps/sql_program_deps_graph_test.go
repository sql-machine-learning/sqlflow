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

package deps

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/go/parser"
)

func TestExtendedSyntaxParseToTrain(t *testing.T) {
	a := assert.New(t)
	sqlProgram := `CREATE TABLE table1 AS SELECT * FROM origin;
	CREATE TABLE table2 AS SELECT * FROM table1;
	SELECT * FROM table2 WHERE a=1;
	SELECT * FROM table2 WHERE a=2;
	DROP TABLE table2;
	DROP TABLE table1;`
	driverType := os.Getenv("SQLFLOW_TEST_DB")
	if driverType == "" {
		driverType = "mysql"
	}
	res, err := parser.Parse(driverType, sqlProgram)
	a.NoError(err)
	Stmts, err := Analyze(res)
	a.NoError(err)
	a.Equal(6, len(Stmts))
	if Stmts[0] != nil {
		// FIXME(typhoonzero): add this test when mysql/hive/calcite parser implemented getting input/output tables.
		// This test is now used for MaxCompute parser.
		a.Equal(0, len(Stmts[0].Outputs))
	}
}
