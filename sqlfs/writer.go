package sqlfs

import (
	"database/sql"
	"fmt"
)

const kBufSize = 4 * 1024

// Writer implements io.WriteCloser.
type Writer struct {
	db    *sql.DB
	table string
	buf   []byte
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
	return &Writer{db, table, make([]byte, 0, kBufSize)}, nil
}

func (w *Writer) Write(p []byte) (n int, e error) {
	n = 0
	for np := len(p); np > 0; np = len(p) {
		fill := kBufSize - len(w.buf)
		if fill > np {
			fill = np
		}
		w.buf = append(w.buf, p[:fill]...)
		p = p[fill:]
		n += fill
		if len(w.buf) >= kBufSize {
			if e = w.flush(); e != nil {
				return n, fmt.Errorf("writer failed: %v", e)
			}
		}
	}
	return n, nil
}

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
		query := fmt.Sprintf("INSERT INTO %s (block) VALUES(?)", w.table)
		if _, e := w.db.Exec(query, w.buf); e != nil {
			return fmt.Errorf("flush to database exec:%v, error:%v", query, e)
		}
		w.buf = w.buf[:0]
	}
	return nil
}
