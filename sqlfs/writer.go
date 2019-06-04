// Copyright 2019 The SQLFlow Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sqlfs

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
)

const bufSize = 4 * 1024

// Writer implements io.WriteCloser.
type Writer struct {
	buf []byte
}

func (w *Writer) flush() error {
	return nil
}

// TableWriter writes to the given table in db
type TableWriter struct {
	Writer
	db      *sql.DB
	table   string
	flushID int
}

// FileWriter writes to the file on the host
type FileWriter struct {
	Writer
	file *os.File
}

// CreateTableWriter creates a new table or truncates an existing table and
// returns a TableWriter.
func CreateTableWriter(db *sql.DB, driver, table string) (*TableWriter, error) {
	if e := dropTable(db, table); e != nil {
		return nil, fmt.Errorf("create: %v", e)
	}
	if e := createTable(db, driver, table); e != nil {
		return nil, fmt.Errorf("create: %v", e)
	}
	return &TableWriter{Writer{make([]byte, 0, bufSize)}, db, table, 0}, nil
}

//CreateFileWriter creates a new file and returns a FileWriter
func CreateFileWriter(fn string) (*FileWriter, error) {
	f, err := os.Create(fn)
	if err != nil {
		return nil, fmt.Errorf("create %v", err)
	}
	return &FileWriter{Writer{make([]byte, 0, bufSize)}, f}, nil
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
func (w *TableWriter) Close() error {
	if e := w.flush(); e != nil {
		return fmt.Errorf("close failed: %v", e)
	}
	w.db = nil // mark closed
	return nil
}

// Close the file handler
func (w *FileWriter) Close() error {
	if e := w.flush(); e != nil {
		return fmt.Errorf("close failed: %v", e)
	}
	w.file.Close()
	return nil
}

func (w *TableWriter) flush() error {
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

func (w *FileWriter) flush() error {
	if w.file == nil {
		return fmt.Errorf("bad file handler status")
	}
	if len(w.buf) > 0 {
		block := base64.StdEncoding.EncodeToString(w.buf)
		if _, e := w.file.WriteString(block); e != nil {
			return fmt.Errorf("flush to %s, error: %v", w.file.Name(), e)
		}
		w.file.Sync()
	}
	return nil
}
