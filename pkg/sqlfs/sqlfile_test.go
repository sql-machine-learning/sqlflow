// Copyright 2019 The SQLFlow Authors. All rights reserved.
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
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	_ "sqlflow.org/gohive"
	"sqlflow.org/sqlflow/pkg/database"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

const testDatabaseName = `sqlfs_test`

var (
	createSQLFSTestingDatabaseOnce sync.Once
)

func createSQLFSTestingDatabase() {
	db := database.GetTestingDBSingleton()
	stmt := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", testDatabaseName)
	if _, e := db.Exec(stmt); e != nil {
		log.Fatalf("Cannot create sqlfs testing database %s: %v", testDatabaseName, e)
	}
}

func TestWriterCreate(t *testing.T) {
	createSQLFSTestingDatabaseOnce.Do(createSQLFSTestingDatabase)
	db := database.GetTestingDBSingleton()
	a := assert.New(t)

	tbl := fmt.Sprintf("%s.unittest%d", testDatabaseName, rand.Int())
	w, e := Create(db.DB, db.DriverName, tbl, getDefaultSession())
	a.NoError(e)
	a.NotNil(w)
	defer w.Close()

	has, e1 := hasTable(db.DB, tbl)
	a.NoError(e1)
	a.True(has)

	a.NoError(dropTable(db.DB, tbl))
}

func TestWriteAndRead(t *testing.T) {
	a := assert.New(t)
	const bufSize = 32 * 1024

	createSQLFSTestingDatabaseOnce.Do(createSQLFSTestingDatabase)

	db := database.GetTestingDBSingleton()

	tbl := fmt.Sprintf("%s.unittest%d", testDatabaseName, rand.Int())

	w, e := Create(db.DB, db.DriverName, tbl, getDefaultSession())
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

	a.NoError(dropTable(db.DB, tbl))
}

// assertNoError prints the error if there is any in TestMain, which
// log doesn't work.
func assertNoErr(e error) {
	if e != nil {
		fmt.Println(e)
		os.Exit(-1)
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

func getDefaultSession() *pb.Session {
	return &pb.Session{
		HiveLocation: "/sqlflow",
		HdfsUser:     "",
		HdfsPass:     "",
	}
}
