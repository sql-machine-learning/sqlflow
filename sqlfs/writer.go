package sqlfs

import (
	"database/sql"
	"encoding/base64"
	"fmt"
)

const bufSize = 4 * 1024

// Writer implements io.WriteCloser.
type Writer struct {
	db      *sql.DB
	table   string
	buf     []byte
	flushID int
}

// Create creates a new table or truncates an existing table and
// returns a writer.
func Create(db *sql.DB, driver, table string) (*Writer, error) {
	if e := dropTable(db, table); e != nil {
		return nil, fmt.Errorf("create: %v", e)
	}
	if e := createTable(db, driver, table); e != nil {
		return nil, fmt.Errorf("create: %v", e)
	}
	return &Writer{db, table, make([]byte, 0, bufSize), 0}, nil
}

// Write write bytes to sqlfs and returns (num_bytes, error)
func (w *Writer) Write(p []byte) (n int, e error) {
	n = 0
	for len(p) > 0 {
		fill := bufSize - len(w.buf)
		if fill > len(p) {
			fill = len(p)
		}
		w.buf = append(w.buf, p[:fill]...)
		p = p[fill:]
		n += fill
		if len(w.buf) >= bufSize {
			if e = w.flush(); e != nil {
				return n, fmt.Errorf("writer flush failed: %v", e)
			}
		}
	}
	return n, nil
}

// Close the connection of the sqlfs
func (w *Writer) Close() error {
	if e := w.flush(); e != nil {
		return fmt.Errorf("close failed: %v", e)
	}
	w.db = nil // mark closed
	return nil
}

func (w *Writer) flush() error {
	if w.db == nil {
		return fmt.Errorf("bad database connection")
	}

	if len(w.buf) > 0 {
		block := base64.StdEncoding.EncodeToString(w.buf)
		query := fmt.Sprintf("INSERT INTO %s (id, block) VALUES(%d, '%s')",
			w.table, w.flushID, block)
		if _, e := w.db.Exec(query); e != nil {
			return fmt.Errorf("flush to %s, error:%v", w.table, e)
		}
		w.buf = w.buf[:0]
		w.flushID++
	}
	return nil
}
