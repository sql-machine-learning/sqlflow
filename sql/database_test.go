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
	a := assert.New(t)
	cfg := &mysql.Config{
		User:                 "root",
		Passwd:               "root",
		Net:                  "tcp",
		Addr:                 "localhost:3306",
		AllowNativePasswords: true,
	}
	db, e := Open(fmt.Sprintf("mysql://%s", cfg.FormatDSN()))
	a.NoError(e)
	defer db.Close()

	_, e = db.Exec("show databases")
	a.NoError(e)
}

func TestDatabaseOpenSQLite3(t *testing.T) {
	a := assert.New(t)
	db, e := Open("sqlite3://test")
	a.NoError(e)
	defer db.Close()
	// TODO: need more tests
}
