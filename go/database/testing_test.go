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
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/go/test"
)

func TestDatabaseGetTestingDBSingleton(t *testing.T) {
	db := GetTestingDBSingleton()
	a := assert.New(t)

	switch dbms := test.GetEnv("SQLFLOW_TEST_DB", "mysql"); dbms {
	case "mysql":
		a.Equal(GetTestingMySQLURL(), db.URL())
	case "hive":
		a.Equal(GetTestingHiveURL(), db.URL())
	case "maxcompute":
		a.Equal(testingMaxComputeURL(), db.URL())
	default:
		a.Fail("Unrecognized environment variable SQLFLOW_TEST_DB %s", dbms)
	}
}

func TestDatabaseTestingMySQLURL(t *testing.T) {
	a := assert.New(t)
	if db := GetTestingDBSingleton(); db.DriverName == "mysql" {
		a.Equal(GetTestingMySQLURL(), db.URL())
	}
}

func TestDatabaseTestingHiveURL(t *testing.T) {
	a := assert.New(t)
	if db := GetTestingDBSingleton(); db.DriverName == "hive" {
		a.Equal(GetTestingHiveURL(), db.URL())
	}
}

func TestDatabaseTestingMaxComputeURL(t *testing.T) {
	a := assert.New(t)
	if db := GetTestingDBSingleton(); db.DriverName == "maxcompute" {
		a.Equal(testingMaxComputeURL(), db.URL())
	}
}
