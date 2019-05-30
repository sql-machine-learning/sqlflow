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

package sql

import (
	"fmt"
	"github.com/sql-machine-learning/sqlflow/sql/testdata"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

var (
	testDB *DB
)

func testMySQLSQL() *DB {
	var e error
	cfg := &mysql.Config{
		User:                 getEnv("SQLFLOW_TEST_DB_MYSQL_USER", "root"),
		Passwd:               getEnv("SQLFLOW_TEST_DB_MYSQL_PASSWD", "root"),
		Net:                  getEnv("SQLFLOW_TEST_DB_MYSQL_NET", "tcp"),
		Addr:                 getEnv("SQLFLOW_TEST_DB_MYSQL_ADDR", "127.0.0.1:3306"),
		AllowNativePasswords: true,
	}
	testDB, e = Open(fmt.Sprintf("mysql://%s", cfg.FormatDSN()))
	assertNoErr(e)
	_, e = testDB.Exec("CREATE DATABASE IF NOT EXISTS sqlflow_models;")
	assertNoErr(e)
	assertNoErr(testdata.Popularize(testDB.DB, testdata.IrisSQL))
	assertNoErr(testdata.Popularize(testDB.DB, testdata.ChurnSQL))
	return testDB
}

func testSQLiteSQL() *DB {
	var e error
	testDB, e = Open("sqlite3://:memory:")
	assertNoErr(e)
	// attach an In-Memory Database in SQLite
	for _, name := range []string{"iris", "churn"} {
		_, e = testDB.Exec(fmt.Sprintf("ATTACH DATABASE ':memory:' AS %s;", name))
		assertNoErr(e)
	}
	assertNoErr(testdata.Popularize(testDB.DB, testdata.IrisSQL))
	assertNoErr(testdata.Popularize(testDB.DB, testdata.ChurnSQL))
	return testDB
}

func testHiveSQL() *DB {
	var e error
	// NOTE: sample dataset is written in
	// https://github.com/sql-machine-learning/gohive/blob/develop/docker/entrypoint.sh#L123
	testDB, e = Open("hive://root:root@localhost:10000/churn")
	assertNoErr(e)
	_, e = testDB.Exec("CREATE DATABASE IF NOT EXISTS sqlflow_models;")
	assertNoErr(e)
	assertNoErr(testdata.Popularize(testDB.DB, testdata.IrisHiveSQL))
	assertNoErr(testdata.Popularize(testDB.DB, testdata.ChurnHiveSQL))
	return testDB
}

func TestMain(m *testing.M) {
	dbms := getEnv("SQLFLOW_TEST_DB", "mysql")
	switch dbms {
	case "sqlite3":
		testDB = testSQLiteSQL()
		defer testDB.Close()
	case "mysql":
		testDB = testMySQLSQL()
		defer testDB.Close()
	case "hive":
		testDB = testHiveSQL()
		defer testDB.Close()
	default:
		e := fmt.Errorf("unrecognized environment variable SQLFLOW_TEST_DB %s", dbms)
		assertNoErr(e)
	}

	os.Exit(m.Run())
}

// assertNoError prints the error if there is any in TestMain, which
// log doesn't work.
func assertNoErr(e error) {
	if e != nil {
		fmt.Println(e)
		os.Exit(-1)
	}
}
