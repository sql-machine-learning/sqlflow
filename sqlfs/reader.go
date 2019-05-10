package sqlfs

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
)

// Reader implements io.ReadCloser
type Reader struct {
	db    *sql.DB
	table string
	buf   []byte
	rows  *sql.Rows
}

// Open returns a reader to read from the given table in db.
func Open(db *sql.DB, table string) (*Reader, error) {
	has, e := hasTable(db, table)
	if !has {
		return nil, fmt.Errorf("open: table %s doesn't exist", table)
	}
	if e != nil {
		return nil, fmt.Errorf("open: hasTable failed with %v", e)
	}

	r := &Reader{db, table, nil, nil}
	// hive need select the id for `order by`
	stmt := fmt.Sprintf("SELECT id,block FROM %s ORDER BY id", table)
	r.rows, e = r.db.Query(stmt)
	if e != nil {
		return nil, fmt.Errorf("open: db query [%s] failed: %v", stmt, e)
	}
	return r, nil
}

func (r *Reader) Read(p []byte) (n int, e error) {
	if r.db == nil {
		return 0, fmt.Errorf("read from a closed reader")
	}
	n = 0
	for n < len(p) {
		m := copy(p[n:], r.buf)
		n += m
		r.buf = r.buf[m:]
		if len(r.buf) <= 0 {
			if r.rows.Next() {
				var block string
				var id int
				if e = r.rows.Scan(&id, &block); e != nil {
					break
				}
				if r.buf, e = base64.StdEncoding.DecodeString(block); e != nil {
					break
				}
			} else {
				e = io.EOF
				break
			}
		}
	}
	return n, e
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
