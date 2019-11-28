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
	"io"
	"io/ioutil"

	pb "sqlflow.org/sqlflow/pkg/proto"
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
func Create(db *sql.DB, driver, table string, session *pb.Session) (io.WriteCloser, error) {
	if e := dropTable(db, table); e != nil {
		return nil, fmt.Errorf("create: %v", e)
	}
	if e := createTable(db, driver, table); e != nil {
		return nil, fmt.Errorf("create: %v", e)
	}

	if driver == "hive" {
		// HiveWriter implement can archive better performance
		csvFile, e := ioutil.TempFile("/tmp", "sqlflow-sqlfs")
		if e != nil {
			return nil, fmt.Errorf("create temporary csv file failed: %v", e)
		}
		return &HiveWriter{
			Writer: Writer{
				db:      db,
				table:   table,
				buf:     make([]byte, 0, bufSize),
				flushID: 0,
			},
			csvFile: csvFile,
			session: session}, nil
	}
	// default writer implement
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
