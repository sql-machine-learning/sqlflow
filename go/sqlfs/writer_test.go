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
	"log"
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	_ "sqlflow.org/gohive"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/test"
)

var testDatabaseName = `sqlfs_test`

var (
	createSQLFSTestingDatabaseOnce sync.Once
)

func createSQLFSTestingDatabase() {
	db := database.GetTestingDBSingleton()
	if db.DriverName == "maxcompute" {
		// maxcompute database is pre created
		testDatabaseName = test.GetEnv("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT", "test")
	} else {
		stmt := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", testDatabaseName)
		if _, e := db.Exec(stmt); e != nil {
			log.Fatalf("Cannot create sqlfs testing database %s: %v", testDatabaseName, e)
		}
	}
}

func TestSQLFSWriterCreate(t *testing.T) {
	createSQLFSTestingDatabaseOnce.Do(createSQLFSTestingDatabase)
	db := database.GetTestingDBSingleton()
	a := assert.New(t)

	tbl := fmt.Sprintf("%s.unittest%d", testDatabaseName, rand.Int())
	w, e := Create(db, tbl, database.GetSessionFromTestingDB())
	a.NoError(e)
	a.NotNil(w)
	defer w.Close()

	has, e1 := hasTable(db.DB, tbl)
	a.NoError(e1)
	a.True(has)

	a.NoError(dropTableIfExists(db.DB, tbl))
}
