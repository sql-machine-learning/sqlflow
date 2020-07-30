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

package alps

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

func mockSession() *pb.Session {
	db := database.GetTestingDBSingleton()
	return &pb.Session{DbConnStr: fmt.Sprintf("%s://%s", db.DriverName, db.DataSourceName)}
}

func TestALPSCodegen(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST_DB") != "maxcompute" {
		t.Skipf("skip TestALPSCodegen and it must use when SQLFLOW_TEST_DB=maxcompute")
	}
	a := assert.New(t)
	tir := ir.MockTrainStmt(false)
	_, err := Train(tir, mockSession())
	a.NoError(err)
}
