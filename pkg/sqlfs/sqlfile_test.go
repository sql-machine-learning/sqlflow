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
	"database/sql"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	_ "sqlflow.org/gohive"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

var (
	testCfg    *mysql.Config
	testDB     *sql.DB
	testDriver string
)

const testDatabaseName = `sqlfs_test`

func TestCreateHasDropTable(t *testing.T) {
	a := assert.New(t)

	fn := fmt.Sprintf("%s.unittest%d", testDatabaseName, rand.Int())
	a.NoError(createTable(testDB, testDriver, fn))
	has, e := hasTable(testDB, fn)
	a.NoError(e)
	a.True(has)
	a.NoError(dropTable(testDB, fn))
}

func TestWriterCreate(t *testing.T) {
	a := assert.New(t)

	fn := fmt.Sprintf("%s.unittest%d", testDatabaseName, rand.Int())
	w, e := Create(testDB, testDriver, fn, getDefaultSession())
	a.NoError(e)
	a.NotNil(w)
	defer w.Close()

	has, e1 := hasTable(testDB, fn)
	a.NoError(e1)
	a.True(has)

	a.NoError(dropTable(testDB, fn))
}

func TestWriteAndRead(t *testing.T) {
	testDriver = getEnv("SQLFLOW_TEST_DB", "mysql")
	a := assert.New(t)

	fn := fmt.Sprintf("%s.unittest%d", testDatabaseName, rand.Int())

	w, e := Create(testDB, testDriver, fn, getDefaultSession())
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

	r, e := Open(testDB, fn)
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

	a.NoError(dropTable(testDB, fn))
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

func TestMain(m *testing.M) {
	testDriver = getEnv("SQLFLOW_TEST_DB", "mysql")

	var e error
	switch testDriver {
	case "mysql":
		cfg := &mysql.Config{
			User:                 getEnv("SQLFLOW_TEST_DB_MYSQL_USER", "root"),
			Passwd:               getEnv("SQLFLOW_TEST_DB_MYSQL_PASSWD", "root"),
			Net:                  getEnv("SQLFLOW_TEST_DB_MYSQL_NET", "tcp"),
			Addr:                 getEnv("SQLFLOW_TEST_DB_MYSQL_ADDR", "127.0.0.1:3306"),
			AllowNativePasswords: true,
		}
		testDB, e = sql.Open("mysql", cfg.FormatDSN())
		assertNoErr(e)
		_, e = testDB.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", testDatabaseName))
		assertNoErr(e)
		defer testDB.Close()
	case "hive":
		testDB, e = sql.Open("hive", "root:root@localhost:10000/churn")
		assertNoErr(e)
		_, e = testDB.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", testDatabaseName))
		assertNoErr(e)
		defer testDB.Close()
	default:
		assertNoErr(fmt.Errorf("unrecognized environment variable SQLFLOW_TEST_DB %s", testDriver))
	}
	os.Exit(m.Run())
}
