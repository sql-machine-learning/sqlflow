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

	pb "sqlflow.org/sqlflow/pkg/proto"
)

// TEXT/STRING field support 64KB maximumn storage size
const bufSize = 32 * 1024

// Writer implements io.WriteCloser.
type Writer struct {
	db      *sql.DB
	table   string
	flushID int
}

func noopWrapUp() error { return nil }

// Create creates a new table or truncates an existing table and
// returns a writer.
func Create(db *sql.DB, driver, table string, session *pb.Session) (io.WriteCloser, error) {
	if e := dropTable(db, table); e != nil {
		return nil, fmt.Errorf("drop table error: %v", e)
	}
	if e := createTable(db, driver, table); e != nil {
		return nil, fmt.Errorf("create table error: %v", e)
	}
	if driver == "hive" {
		w, e := NewHiveWriter(db, table, session)
		if e != nil {
			return nil, fmt.Errorf("create HiveWriter error: %v", e)
		}
		return newFlushWriteCloser(flushToCSV(w), uploadHDFSWrapUp(w), bufSize), nil
	}
	w := &Writer{db, table, 0}
	return newFlushWriteCloser(flushToTable(w), noopWrapUp, bufSize), nil
}

func flushToTable(w *Writer) func([]byte, int) error {
	return func(buf []byte, flushes int) error {
		if w.db == nil {
			return fmt.Errorf("bad database connection")
		}
		block := base64.StdEncoding.EncodeToString(buf)
		query := fmt.Sprintf("INSERT INTO %s (id, block) VALUES(%d, '%s')",
			w.table, flushes, block)
		if _, e := w.db.Exec(query); e != nil {
			return fmt.Errorf("flush to %s, error:%v", w.table, e)
		}
		return nil
	}
}
