package sqlfs

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// createTable creates a table, if it doesn't exist.  If the table
// name includes the database name, e.g., "db.tbl", it creates the
// database if necessary.
func CreateTable(db *sql.DB, table string) error {
	if ss := strings.Split(table, "."); len(ss) > 1 {
		stmt := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;",
			strings.Join(ss[:len(ss)-1], "."))

		if _, e := db.Exec(stmt); e != nil {
			return fmt.Errorf("createTable %s: %v", stmt, e)
		}
	}

	stmt := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (block BLOB)", table)
	if _, e := db.Exec(stmt); e != nil {
		return fmt.Errorf("createTable cannot create table %s: %v", table, e)
	}

	// NOTE: a double-check of HasTable is necessary. For example,
	// MySQL doesn't allow '-' in table names; however, if there
	// is, the db.Exec wouldn't return any error.
	has, e1 := HasTable(db, table)
	if e1 != nil {
		return fmt.Errorf("createTable cannot verify the creation: %v", e1)
	}
	if !has {
		return fmt.Errorf("createTable verified table not created")
	}
	return nil
}

// DropTable removes a table if it exists.  If the table name includes
// the database name, e.g., "db.tbl", it doesn't try to remove the
// database.
func DropTable(db *sql.DB, table string) error {
	stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", table)
	if _, e := db.Exec(stmt); e != nil {
		return fmt.Errorf("dropTable %s: %v", table, e)
	}
	return nil
}

// HasTable checks if a table exists.
func HasTable(db *sql.DB, table string) (bool, error) {
	if _, e := db.Exec("DESCRIBE " + table); e != nil {
		// MySQL error 1146 is "table does not exist"
		if mErr, ok := e.(*mysql.MySQLError); ok && mErr.Number == 1146 {
			return false, nil
		}
		return false, fmt.Errorf("HasTable DESCRIBE %s failed: %v", table, e)
	}
	return true, nil
}
