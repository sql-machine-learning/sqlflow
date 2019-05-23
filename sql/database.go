package sql

import (
	"database/sql"
	"fmt"
	"strings"

	// import drivers for heterogonous DB support
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	_ "sqlflow.org/gohive"
	_ "sqlflow.org/gomaxcompute"
)

const driverSeparator = "://"

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
	sep := strings.Index(datasource, driverSeparator)
	if sep == -1 {
		return nil, fmt.Errorf("bad datasource, driver name is missing")
	}
	db := &DB{
		driverName:     datasource[:sep],
		dataSourceName: datasource[sep+len(driverSeparator):],
	}

	var err error
	switch db.driverName {
	case "sqlite3", "mysql", "hive", "maxcompute":
		db.DB, err = sql.Open(db.driverName, db.dataSourceName)
	default:
		return nil, fmt.Errorf("sqlfow currently doesn't support DB %v", db.driverName)
	}
	return db, err
}
