package sqlfile

import (
	"database/sql"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

// Reader implements io.ReadCloser
type Reader struct {
	db    *sql.DB
	table string
	buf   []byte
	rows  *sql.Rows
}

func Open(db *sql.DB, table string) (*Reader, error) {
	has, e := hasTable(db, table)
	if !has {
		return nil, fmt.Errorf("Open: table %s doesn't exist", table)
	}
	if e != nil {
		return nil, fmt.Errorf("Open: hasTable failed with %v", e)
	}

	r := &Reader{
		db:    db,
		table: table,
		buf:   make([]byte, 0, kMaxRowSize),
		rows:  nil}

	r.rows, e = r.db.Query(fmt.Sprintf("SELECT block FROM %s", table))
	if e != nil {
		return nil, fmt.Errorf("Open: failed to query: %v", e)
	}
	return r, nil
}

func (r *Reader) Read(p []byte) (n int, e error) {
	n = 0
	for n < len(p) {
		m := copy(p[n:], r.buf)
		n += m
		r.buf = r.buf[m:]
		if len(r.buf) <= 0 {
			if r.rows.Next() {
				e := r.rows.Scan(&r.buf)
				if e != nil {
					return n, e
				}
			}
		}
	}
	return n, nil
}

func (r *Reader) Close() error {
	if r.rows != nil {
		if e := r.rows.Close(); e != nil {
			return e
		}
		r.rows = nil
	}
	r.db = nil // Mark closed.
	return nil
}

// hasTable checks if a table exists.  We took https://bit.ly/2DvCIO1
// as a reference.
func hasTable(db *sql.DB, table string) (bool, error) {
	if _, e := db.Exec("DESCRIBE " + table); e != nil {
		// MySQL error 1146 is "table does not exist"
		if mErr, ok := e.(*mysql.MySQLError); ok && mErr.Number == 1146 {
			return false, nil
		}
		return false, fmt.Errorf("hasTable DESCRIBE %s failed: %v", table, e)
	}
	return true, nil
}
