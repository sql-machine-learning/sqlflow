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

package database

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"sqlflow.org/gomaxcompute"
	"sqlflow.org/sqlflow/go/sql/testdata"
	"sqlflow.org/sqlflow/go/test"

	"github.com/go-sql-driver/mysql"
	"sqlflow.org/sqlflow/go/proto"
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
	switch dbms := test.GetEnv("SQLFLOW_TEST_DB", "mysql"); dbms {
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

// GetTestingMySQLConfig construct a MySQL config
func GetTestingMySQLConfig() *mysql.Config {
	return &mysql.Config{
		User:                 test.GetEnv("SQLFLOW_TEST_DB_MYSQL_USER", "root"),
		Passwd:               test.GetEnv("SQLFLOW_TEST_DB_MYSQL_PASSWD", "root"),
		Net:                  test.GetEnv("SQLFLOW_TEST_DB_MYSQL_NET", "tcp"),
		Addr:                 test.GetEnv("SQLFLOW_TEST_DB_MYSQL_ADDR", "127.0.0.1:3306"),
		AllowNativePasswords: true,
	}
}

// GetTestingMySQLURL returns MySQL connection URL
func GetTestingMySQLURL() string {
	return fmt.Sprintf("mysql://%s", GetTestingMySQLConfig().FormatDSN())
}

func createTestingMySQLDB() *DB {
	db, e := OpenAndConnectDB(GetTestingMySQLURL())
	assertNoErr(e)
	_, e = db.Exec("CREATE DATABASE IF NOT EXISTS sqlflow_models;")
	assertNoErr(e)
	assertNoErr(testdata.Popularize(db.DB, testdata.IrisSQL))
	assertNoErr(testdata.Popularize(db.DB, testdata.SanityCheckSQL))
	assertNoErr(testdata.Popularize(db.DB, testdata.ChurnSQL))
	assertNoErr(testdata.Popularize(db.DB, testdata.HousingSQL))
	return db
}

// GetTestingHiveURL reutrns Hive connection URL
func GetTestingHiveURL() string {
	// NOTE: sample dataset is written in
	// https://github.com/sql-machine-learning/gohive/blob/develop/docker/entrypoint.sh#L123
	namenodeAddr := os.Getenv("SQLFLOW_TEST_HDFS_NAMENODE_ADDR")
	hiveLocation := os.Getenv("SQLFLOW_HIVE_LOCATION")
	if hiveLocation == "" {
		hiveLocation = "/sqlflow"
	}
	return fmt.Sprintf("hive://root:root@localhost:10000/iris?"+
		"hdfs_namenode_addr=%s&hive_location=%s", namenodeAddr, hiveLocation)
}

func createTestingHiveDB() *DB {
	db, e := OpenAndConnectDB(GetTestingHiveURL())
	assertNoErr(e)
	_, e = db.Exec("CREATE DATABASE IF NOT EXISTS sqlflow_models;")
	assertNoErr(e)
	assertNoErr(testdata.Popularize(db.DB, testdata.IrisHiveSQL))
	assertNoErr(testdata.Popularize(db.DB, testdata.ChurnHiveSQL))
	assertNoErr(testdata.Popularize(db.DB, testdata.HousingSQL))
	return db
}

func testingMaxComputeConfig() *gomaxcompute.Config {
	endpoint := test.GetEnv("SQLFLOW_TEST_DB_MAXCOMPUTE_ENDPOINT", "http://service-maxcompute.com/api")
	urlAndArgs := strings.Split(endpoint, "?")
	// accept format like http://service-maxcompute.com/api
	// if got service-maxcompute.com/api?curr_project=xxxx&scheme=http, reformat to http://service-maxcompute.com/api
	if len(urlAndArgs) == 2 {
		if !(strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://")) {
			argList := strings.Split(urlAndArgs[1], "&")
			for _, kv := range argList {
				kvList := strings.Split(kv, "=")
				if len(kvList) == 2 && kvList[0] == "scheme" {
					endpoint = fmt.Sprintf("%s://%s", kvList[1], urlAndArgs[0])
					break
				}
			}
		}
	} else {
		if !(strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://")) {
			log.Fatal("SQLFLOW_TEST_DB_MAXCOMPUTE_ENDPOINT must be the form of http://service-maxcompute.com/api or service-maxcompute.com/api?scheme=http")
		}
	}
	return &gomaxcompute.Config{
		AccessID:  test.GetEnv("SQLFLOW_TEST_DB_MAXCOMPUTE_AK", "test"),
		AccessKey: test.GetEnv("SQLFLOW_TEST_DB_MAXCOMPUTE_SK", "test"),
		Project:   test.GetEnv("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT", "test"),
		Endpoint:  endpoint,
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
		DbConnStr: db.URL()}
}
