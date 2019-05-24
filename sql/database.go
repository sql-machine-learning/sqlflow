package sql

import (
	"database/sql"
	"fmt"
	"strings"

	// import drivers for heterogonous DB support
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	_ "sqlflow.org/gohive"
)

// DB extends sql.DB
type DB struct {
	driverName     string
	dataSourceName string
	*sql.DB
}

// Open pases a datasource string into driver name and datasource name,
// then opens a database specified by the driver name and a driver-specific
// data source name, usually consisting of at least a database name and
// connection information.
//
// In addition to sql.Open, it also does the book keeping on driverName and
// dataSourceName
func Open(datasource string) (*DB, error) {
	dses := strings.Split(datasource, "://")
	if len(dses) != 2 {
		return nil, fmt.Errorf("Expecting but cannot find :// in datasource %v", datasource)
	}
	db := &DB{driverName: dses[0], dataSourceName: dses[1]}

	var err error
	switch db.driverName {
	case "sqlite3", "mysql", "hive":
		db.DB, err = sql.Open(db.driverName, db.dataSourceName)
	default:
		return nil, fmt.Errorf("sqlfow currently doesn't support DB %v", db.driverName)
	}
	return db, err
}
