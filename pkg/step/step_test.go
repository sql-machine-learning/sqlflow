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
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/tablewriter"
)

func makeTestSession(dbConnStr string) *pb.Session {
	return &pb.Session{DbConnStr: dbConnStr}
}

func trainLogChecker(s string) error {
	expectLog := "Done training"
	if strings.Contains(s, expectLog) {
		return nil
	}
	return fmt.Errorf("train sql log does not contain the expected content: %s", expectLog)
}
func TestStepTrainSQL(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST_DB") != "mysql" {
		t.Skip("skip no mysql test.")
	}
	a := assert.New(t)
	dbConnStr := "mysql://root:root@tcp(127.0.0.1:3306)/iris?maxAllowedPacket=0"
	session := makeTestSession(dbConnStr)

	sql := `SELECT * FROM iris.train WHERE class!=2
	TO TRAIN DNNClassifier
	WITH
		model.n_classes = 2,
		model.hidden_units = [10, 10],
		train.batch_size = 4,
		validation.select = "SELECT * FROM iris.test WHERE class!=2",
		validation.metrics = "Accuracy,AUC"
	LABEL class
	INTO sqlflow_models.mytest_model;`
	table, e := tablewriter.Create("ascii", 100, os.Stdout)
	a.NoError(e)
	out, e := GetStdout(func() error { return RunSQLProgramAndPrintResult(sql, "", session, table, false, false) })
	a.NoError(e)
	a.NoError(trainLogChecker(out))
}
