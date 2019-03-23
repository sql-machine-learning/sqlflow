package sqlfs

import (
	"database/sql"
	"fmt"
)

var kBufSize = 4 * 1024

// Writer implements io.WriteCloser.
type Writer struct {
	db     *sql.DB
	table  string
	buf    []byte
	insert *sql.Stmt
}

// Create creates a new table or truncates an existing table and
// returns a writer.
func Create(db *sql.DB, table string) (*Writer, error) {
	if e := dropTable(db, table); e != nil {
		return nil, fmt.Errorf("create: %v", e)
	}
	return Append(db, table)
}

// Append returns a writer to append to an existing table.  It creates
// the table if it doesn't exist.
func Append(db *sql.DB, table string) (*Writer, error) {
	if e := createTable(db, table); e != nil {
		return nil, fmt.Errorf("create: %v", e)
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
				return n, fmt.Errorf("writer flush failed: %v", e)
			}
		}
	}
	return n, nil
}

func (w *Writer) Close() error {
	if e := w.flush(); e != nil {
		return fmt.Errorf("writer flush failed: %v", e)
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
