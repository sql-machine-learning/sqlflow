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

package database

import (
	"fmt"
	"log"
	"os"
	"sync"

	"sqlflow.org/gomaxcompute"
	"sqlflow.org/sqlflow/pkg/sql/testdata"

	"github.com/go-sql-driver/mysql"
	"sqlflow.org/sqlflow/pkg/proto"
)

var (
	muTestingDBSingleton sync.Mutex
	testingDBSingleton   *DB
)

// GetTestingDBSingleton returns the testing DB singleton.
func GetTestingDBSingleton() *DB {
	muTestingDBSingleton.Lock()
	defer muTestingDBSingleton.Unlock()

	if testingDBSingleton == nil {
		testingDBSingleton = createTestingDB()
	}
	return testingDBSingleton
}

// createTestingDB opens a database with parameters specified in the
// environment variables with prefix name "SQLFLOW_TEST_DB".  By
// default, the database driver is "mysql".  It also creates some
// tables in the opened database, and popularize data.  For any error,
// createTestingDB panics.
//
// NOTE: It is the caller's responsibility to close the database.  In
// order to do it, users might want to define TestMain and call
// createTestingDB and defer db.Close in it.
func createTestingDB() *DB {
	switch dbms := getEnv("SQLFLOW_TEST_DB", "mysql"); dbms {
	case "mysql":
		return createTestingMySQLDB()
	case "hive":
		return createTestingHiveDB()
	case "maxcompute":
		return createTestingMaxComputeDB()
	default:
		log.Panicf("Unrecognized environment variable SQLFLOW_TEST_DB %s", dbms)
	}
	return nil
}

func getEnv(env, value string) string {
	if env := os.Getenv(env); len(env) != 0 {
		return env
	}
	return value
}

func testingMySQLConfig() *mysql.Config {
	return &mysql.Config{
		User:                 getEnv("SQLFLOW_TEST_DB_MYSQL_USER", "root"),
		Passwd:               getEnv("SQLFLOW_TEST_DB_MYSQL_PASSWD", "root"),
		Net:                  getEnv("SQLFLOW_TEST_DB_MYSQL_NET", "tcp"),
		Addr:                 getEnv("SQLFLOW_TEST_DB_MYSQL_ADDR", "127.0.0.1:3306"),
		AllowNativePasswords: true,
	}
}

func testingMySQLURL() string {
	return fmt.Sprintf("mysql://%s", testingMySQLConfig().FormatDSN())
}

func createTestingMySQLDB() *DB {
	db, e := OpenAndConnectDB(testingMySQLURL())
	assertNoErr(e)
	_, e = db.Exec("CREATE DATABASE IF NOT EXISTS sqlflow_models;")
	assertNoErr(e)
	assertNoErr(testdata.Popularize(db.DB, testdata.IrisSQL))
	assertNoErr(testdata.Popularize(db.DB, testdata.SanityCheckSQL))
	assertNoErr(testdata.Popularize(db.DB, testdata.ChurnSQL))
	assertNoErr(testdata.Popularize(db.DB, testdata.HousingSQL))
	return db
}

func testingHiveURL() string {
	// NOTE: sample dataset is written in
	// https://github.com/sql-machine-learning/gohive/blob/develop/docker/entrypoint.sh#L123
	return "hive://root:root@localhost:10000/churn"
}

func createTestingHiveDB() *DB {
	db, e := OpenAndConnectDB(testingHiveURL())
	assertNoErr(e)
	_, e = db.Exec("CREATE DATABASE IF NOT EXISTS sqlflow_models;")
	assertNoErr(e)
	assertNoErr(testdata.Popularize(db.DB, testdata.IrisHiveSQL))
	assertNoErr(testdata.Popularize(db.DB, testdata.ChurnHiveSQL))
	assertNoErr(testdata.Popularize(db.DB, testdata.HousingSQL))
	return db
}

func testingMaxComputeConfig() *gomaxcompute.Config {
	return &gomaxcompute.Config{
		AccessID:  os.Getenv("MAXCOMPUTE_AK"),
		AccessKey: os.Getenv("MAXCOMPUTE_SK"),
		Project:   os.Getenv("MAXCOMPUTE_PROJECT"),
		Endpoint:  os.Getenv("MAXCOMPUTE_ENDPOINT"),
	}
}

func testingMaxComputeURL() string {
	return fmt.Sprintf("maxcompute://%s", testingMaxComputeConfig().FormatDSN())
}

func createTestingMaxComputeDB() *DB {
	db, e := OpenAndConnectDB(testingMaxComputeURL())
	assertNoErr(e)
	// Note: We do not popularize the test data here intentionally since
	// it will take up quite some time on MaxCompute.
	return db
}

// assertNoError prints the error if there is any in TestMain, which
// log doesn't work.
func assertNoErr(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

// GetSessionFromTestingDB construct a proto message Session
// representing the testing database configuration.
func GetSessionFromTestingDB() *proto.Session {
	db := GetTestingDBSingleton()
	return &proto.Session{
		DbConnStr:    db.URL(),
		HiveLocation: "/sqlflow",
		HdfsUser:     "",
		HdfsPass:     ""}
}
