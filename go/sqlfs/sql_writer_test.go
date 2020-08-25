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
	"io"
	"math/rand"
	"strings"
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

func TestSQLFSSQLWriterWriteAndRead(t *testing.T) {
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

	// A small output.
	buf := []byte("\n\n\n")
	n, e := w.Write(buf)
	a.NoError(e)
	a.Equal(len(buf), n)

	// A big output.
	buf = make([]byte, bufSize+1)
	for i := range buf {
		buf[i] = 'x'
	}
	n, e = w.Write(buf)
	a.NoError(e)
	a.Equal(len(buf), n)

	a.NoError(w.Close())

	r, e := Open(db.DB, tbl)
	a.NoError(e)
	a.NotNil(r)

	// A small read
	buf = make([]byte, 2)
	n, e = r.Read(buf)
	a.NoError(e)
	a.Equal(2, n)
	a.Equal(2, strings.Count(string(buf), "\n"))

	// A big read of rest
	buf = make([]byte, bufSize*2)
	n, e = r.Read(buf)
	a.Equal(io.EOF, e)
	a.Equal(bufSize+2, n)
	a.Equal(1, strings.Count(string(buf), "\n"))
	a.Equal(bufSize+1, strings.Count(string(buf), "x"))

	// Another big read
	n, e = r.Read(buf)
	a.Equal(io.EOF, e)
	a.Equal(0, n)
	a.NoError(r.Close())

	a.NoError(dropTableIfExists(db.DB, tbl))
}
