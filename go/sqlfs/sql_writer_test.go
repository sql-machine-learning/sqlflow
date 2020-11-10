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

package sqlfs

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/go/database"
)

func TestSQLFSNewSQLWriter(t *testing.T) {
	createSQLFSTestingDatabaseOnce.Do(createSQLFSTestingDatabase)
	db := database.GetTestingDBSingleton()
	a := assert.New(t)

	if db.DriverName == "hive" {
		t.Skip("Skip as SQLFLOW_TEST_DB is Hive")
	}
	t.Logf("Confirm executed with %s", db.DriverName)

	tbl := fmt.Sprintf("%s.unittest%d", testDatabaseName, rand.Int())
	w, e := newSQLWriter(db, tbl, bufSize)
	a.NoError(e)
	a.NotNil(w)
	defer w.Close()

	has, e1 := hasTable(db.DB, tbl)
	a.NoError(e1)
	a.True(has)

	a.NoError(dropTableIfExists(db.DB, tbl))
}
