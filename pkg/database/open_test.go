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
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
)

func TestDatabaseOpenMysql(t *testing.T) {
	if getEnv("SQLFLOW_TEST_DB", "mysql") == "hive" {
		t.Skip("hive: skip MySQL test")
	}
	a := assert.New(t)
	cfg := &mysql.Config{
		User:                 "root",
		Passwd:               "root",
		Net:                  "tcp",
		Addr:                 "localhost:3306",
		AllowNativePasswords: true,
	}
	connStr := fmt.Sprintf("mysql://%s", cfg.FormatDSN())
	db, e := NewDB(connStr)
	a.NoError(e)
	defer db.Close()

	a.EqualValues(connStr, db.String())
	_, e = db.Exec("show databases")
	a.NoError(e)
}

func TestSplitDataSource(t *testing.T) {
	a := assert.New(t)
	ds := "mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0"
	driverName, datasourceName, e := SplitDataSource(ds)
	a.EqualValues(driverName, "mysql")
	a.EqualValues(datasourceName, "root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0")
	a.NoError(e)
}
