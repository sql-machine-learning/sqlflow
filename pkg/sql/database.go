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
	"database/sql"
	"fmt"
	"strings"

	// import drivers for heterogenous DB support
	_ "github.com/go-sql-driver/mysql"
	_ "sqlflow.org/gohive"
	_ "sqlflow.org/gomaxcompute"
)

// DB extends sql.DB
type DB struct {
	driverName     string
	dataSourceName string
	*sql.DB
}

// open passes a datasource string into driver name and datasource name,
// then opens a database specified by the driver name and a driver-specific
// data source name, usually consisting of at least a database name and
// connection information.
//
// In addition to sql.Open, it also does the book keeping on driverName and
// dataSourceName
func open(datasource string) (*DB, error) {
	driverName, datasourName, err := SplitDataSource(datasource)
	if err != nil {
		return nil, err
	}
	db := &DB{driverName: driverName, dataSourceName: datasourName}

	err = openDB(db)
	return db, err
}

func openDB(db *DB) error {
	var err error
	for _, d := range sql.Drivers() {
		if db.driverName == d {
			db.DB, err = sql.Open(db.driverName, db.dataSourceName)
			if err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("sqlflow currently doesn't support DB %s", db.driverName)
}

// SplitDataSource splits the datasource into drivername and datasource name
func SplitDataSource(datasource string) (string, string, error) {
	if datasource == "" {
		return "", "", fmt.Errorf("datasource should not be an empty string")
	}
	dses := strings.Split(datasource, "://")
	if len(dses) != 2 {
		return "", "", fmt.Errorf("Expecting but cannot find :// in datasource %v", datasource)
	}
	return dses[0], dses[1], nil
}

// NewDB returns a DB object with verifying the datasource name.
func NewDB(datasource string) (*DB, error) {
	db, err := open(datasource)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}
	return db, nil
}

func (db *DB) String() string {
	return db.driverName + "://" + db.dataSourceName
}
