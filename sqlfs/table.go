package sqlfs

import (
	"database/sql"
	"fmt"
)

// createTable creates a table, if it doesn't exist.  If the table
// name includes the database name, e.g., "db.tbl", it creates the
// database if necessary.
func createTable(db *sql.DB, table string) error {
	// FIXME(tony): HIVE and ODPS don't support AUTO_INCREMENT
	// FIXME(tony): Hive and ODPS don't support BLOB, use BINARY instead
	stmt := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id INT AUTO_INCREMENT, block BLOB, PRIMARY KEY (id))", table)
	if _, e := db.Exec(stmt); e != nil {
		return fmt.Errorf("createTable cannot create table %s: %v", table, e)
	}

	// NOTE: a double-check of hasTable is necessary. For example,
	// MySQL doesn't allow '-' in table names; however, if there
	// is, the db.Exec wouldn't return any error.
	has, e1 := hasTable(db, table)
	if e1 != nil {
		return fmt.Errorf("createTable cannot verify the creation: %v", e1)
	}
	if !has {
		return fmt.Errorf("createTable verified table not created")
	}
	return nil
}

// dropTable removes a table if it exists.  If the table name includes
// the database name, e.g., "db.tbl", it doesn't try to remove the
// database.
func dropTable(db *sql.DB, table string) error {
	stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", table)
	if _, e := db.Exec(stmt); e != nil {
		return fmt.Errorf("dropTable %s: %v", table, e)
	}
	return nil
}

// hasTable checks if a table exists.
func hasTable(db *sql.DB, table string) (bool, error) {
	if _, e := db.Exec(fmt.Sprintf("SELECT 1 FROM %s LIMIT 1;", table)); e != nil {
		return false, fmt.Errorf("hasTable %s failed: %v", table, e)
	}
	return true, nil
}
