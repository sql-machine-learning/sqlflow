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
	"os"
	"testing"

	"sqlflow.org/gomaxcompute"
	"sqlflow.org/sqlflow/pkg/sql/testdata"

	"github.com/go-sql-driver/mysql"
)

var (
	testDB *DB
)

func testMySQLDatabase() *DB {
	cfg := &mysql.Config{
		User:                 getEnv("SQLFLOW_TEST_DB_MYSQL_USER", "root"),
		Passwd:               getEnv("SQLFLOW_TEST_DB_MYSQL_PASSWD", "root"),
		Net:                  getEnv("SQLFLOW_TEST_DB_MYSQL_NET", "tcp"),
		Addr:                 getEnv("SQLFLOW_TEST_DB_MYSQL_ADDR", "127.0.0.1:3306"),
		AllowNativePasswords: true,
	}
	db, e := NewDB(fmt.Sprintf("mysql://%s", cfg.FormatDSN()))
	assertNoErr(e)
	_, e = db.Exec("CREATE DATABASE IF NOT EXISTS sqlflow_models;")
	assertNoErr(e)
	assertNoErr(testdata.Popularize(db.DB, testdata.IrisSQL))
	assertNoErr(testdata.Popularize(db.DB, testdata.SanityCheckSQL))
	assertNoErr(testdata.Popularize(db.DB, testdata.ChurnSQL))
	assertNoErr(testdata.Popularize(db.DB, testdata.HousingSQL))
	return db
}

func testHiveDatabase() *DB {
	// NOTE: sample dataset is written in
	// https://github.com/sql-machine-learning/gohive/blob/develop/docker/entrypoint.sh#L123
	db, e := NewDB("hive://root:root@localhost:10000/churn")
	assertNoErr(e)
	_, e = db.Exec("CREATE DATABASE IF NOT EXISTS sqlflow_models;")
	assertNoErr(e)
	assertNoErr(testdata.Popularize(db.DB, testdata.IrisHiveSQL))
	assertNoErr(testdata.Popularize(db.DB, testdata.ChurnHiveSQL))
	assertNoErr(testdata.Popularize(db.DB, testdata.HousingSQL))
	return db
}

func testMaxcompute() *DB {
	cfg := &gomaxcompute.Config{
		AccessID:  os.Getenv("MAXCOMPUTE_AK"),
		AccessKey: os.Getenv("MAXCOMPUTE_SK"),
		Project:   os.Getenv("MAXCOMPUTE_PROJECT"),
		Endpoint:  os.Getenv("MAXCOMPUTE_ENDPOINT"),
	}

	db, e := NewDB(fmt.Sprintf("maxcompute://%s", cfg.FormatDSN()))
	assertNoErr(e)
	// Note: We do not popularize the test data here intentionally since
	// it will take up quite some time on Maxcompute.
	return db
}

func TestMain(m *testing.M) {
	dbms := getEnv("SQLFLOW_TEST_DB", "mysql")
	switch dbms {
	case "mysql":
		testDB = testMySQLDatabase()
	case "hive":
		testDB = testHiveDatabase()
	case "maxcompute":
		testDB = testMaxcompute()
	default:
		e := fmt.Errorf("unrecognized environment variable SQLFLOW_TEST_DB %s", dbms)
		assertNoErr(e)
	}

	if testDB != nil {
		defer testDB.Close()
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
