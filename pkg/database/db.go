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
	"database/sql"
	"fmt"
	"strings"

	// import drivers for heterogenous DB support
	_ "github.com/go-sql-driver/mysql"
	_ "sqlflow.org/goalisa"
	_ "sqlflow.org/gohive"
	_ "sqlflow.org/gomaxcompute"
)

// DB extends sql.DB, while keeping the two parameters, DriverName and
// DataSource, to database/sql.Open reaccessible.
type DB struct {
	DriverName     string // NOTE: Don't name it Driver, because Driver is a method of sql.DB.
	DataSourceName string
	*sql.DB
}

// OpenDB open a database identified by an URL.  It calls ParseURL
// to get the driver and data source name.  In addition to opening the
// database, it also verifies the driver is loaded.
func OpenDB(url string) (*DB, error) {
	driver, dataSource, err := ParseURL(url)
	if err != nil {
		return nil, err
	}
	db := &DB{DriverName: driver, DataSourceName: dataSource}

	for _, d := range sql.Drivers() {
		if db.DriverName == d {
			db.DB, err = sql.Open(db.DriverName, db.DataSourceName)
			if err != nil {
				return db, err
			}
			return db, nil
		}
	}
	return db, fmt.Errorf("sqlflow currently doesn't support DB %s", db.DriverName)
}

// ParseURL splits the URL into Drivername and DataSourceName.
func ParseURL(url string) (string, string, error) {
	if url == "" {
		return "", "", fmt.Errorf("dataSource should not be an empty string")
	}
	ss := strings.Split(url, "://")
	if len(ss) != 2 {
		return "", "", fmt.Errorf("Expecting but cannot find :// in dataSource %v", url)
	}
	return ss[0], ss[1], nil
}

// unparseURL returns a URL forming of driver and source.
func unparseURL(driver, source string) string {
	return fmt.Sprintf("%s://%s", driver, source)
}

// OpenAndConnectDB calls OpenDB to open a database specified by an
// URL.  In addition to opening, it also call database.DB.Ping to
// ensure a connection to the database.
func OpenAndConnectDB(url string) (*DB, error) {
	db, err := OpenDB(url)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}
	return db, nil
}

// URL calls unparseURL to build the URL from DriverName and
// DataSourceName.
func (db *DB) URL() string {
	return unparseURL(db.DriverName, db.DataSourceName)
}
