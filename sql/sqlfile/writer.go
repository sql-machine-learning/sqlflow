package sqlfile

import (
	"database/sql"
	"fmt"
	"strings"
)

var kBufSize = 4 * 1024

// Writer implements io.WriteCloser.
type Writer struct {
	db     *sql.DB
	table  string
	buf    []byte
	insert *sql.Stmt
}

func Create(db *sql.DB, table string) (*Writer, error) {
	if e := dropTable(db, table); e != nil {
		return nil, fmt.Errorf("Create: %v", e)
	}
	return Append(db, table)
}

func Append(db *sql.DB, table string) (*Writer, error) {
	if e := createTable(db, table); e != nil {
		return nil, fmt.Errorf("Create: %v", e)
	}
	return &Writer{db, table, make([]byte, 0, kBufSize), nil}, nil
}

func (w *Writer) Write(p []byte) (n int, e error) {
	n = 0
	for len(p) > 0 {
		fill := kBufSize - len(w.buf)
		if fill > len(p) {
			fill = len(p)
		}
		w.buf = append(w.buf, p[:fill]...)
		p = p[fill:]
		n += fill
		if len(w.buf) >= kBufSize {
			if e = w.flush(); e != nil {
				return n, fmt.Errorf("Writer flush failed: %v", e)
			}
		}
	}
	return n, nil
}

func (w *Writer) Close() error {
	if e := w.flush(); e != nil {
		return fmt.Errorf("Writer flush failed: %v", e)
	}
	if w.insert != nil {
		if e := w.insert.Close(); e != nil {
			return e
		}
		w.insert = nil // Mark closed.
	}
	w.db = nil // Mark closed.
	return nil
}

func (w *Writer) flush() error {
	var e error
	if w.insert == nil {
		w.insert, e = w.db.Prepare(
			fmt.Sprintf("INSERT INTO %s (block) VALUES(?)", w.table))
		if e != nil {
			return fmt.Errorf("flush failed to prepare insert %s: %v", w.table, e)
		}
	}
	_, e = w.insert.Exec(w.buf)
	if e != nil {
		return fmt.Errorf("flush failed to execute insert: %v", e)
	}
	w.buf = w.buf[:0]
	return nil
}

// createTable creates the table, and if necessary, the database.  We
// took https://bit.ly/2FtkQps as a reference.
func createTable(db *sql.DB, table string) error {
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

func dropTable(db *sql.DB, table string) error {
	stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", table)
	if _, e := db.Exec(stmt); e != nil {
		return fmt.Errorf("dropTable %s: %v", table, e)
	}
	return nil
}
