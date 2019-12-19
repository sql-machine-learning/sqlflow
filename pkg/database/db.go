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
	"database/sql"
	"fmt"
	"strings"

	// import drivers for heterogenous DB support
	_ "github.com/go-sql-driver/mysql"
	_ "sqlflow.org/gohive"
	_ "sqlflow.org/gomaxcompute"
)

// DB extends sql.DB, while keeping the two parameters, Driver and
// DataSoruce, to database/sql.Open reaccessible.
type DB struct {
	Driver     string
	DataSource string
	*sql.DB
}

// open passes a DataSource string into driver name and DataSource name,
// then opens a database specified by the driver name and a driver-specific
// data source name, usually consisting of at least a database name and
// connection information.
//
// In addition to sql.Open, it also does the book keeping on driver and
// DataSource
func open(dataSource string) (*DB, error) {
	driver, dataSource, err := SplitDataSource(dataSource)
	if err != nil {
		return nil, err
	}
	db := &DB{Driver: driver, DataSource: dataSource}

	err = openDB(db)
	return db, err
}

func openDB(db *DB) error {
	var err error
	for _, d := range sql.Drivers() {
		if db.Driver == d {
			db.DB, err = sql.Open(db.Driver, db.DataSource)
			if err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("sqlflow currently doesn't support DB %s", db.Driver)
}

// SplitDataSource splits the DataSource into drivername and DataSource name
func SplitDataSource(dataSource string) (string, string, error) {
	if dataSource == "" {
		return "", "", fmt.Errorf("dataSource should not be an empty string")
	}
	dses := strings.Split(dataSource, "://")
	if len(dses) != 2 {
		return "", "", fmt.Errorf("Expecting but cannot find :// in dataSource %v", dataSource)
	}
	return dses[0], dses[1], nil
}

// NewDB returns a DB object with verifying the dataSource name.
func NewDB(dataSource string) (*DB, error) {
	db, err := open(dataSource)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}
	return db, nil
}

func (db *DB) String() string {
	return db.Driver + "://" + db.DataSource
}
