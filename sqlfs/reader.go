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
	"bufio"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type fragment struct {
	id    int
	block string
}

// Reader implements io.ReadCloser
type Reader struct {
	buf   []byte
	frams []fragment
	cur   int
}

// TableReader implements Reader reads from the given table in db
type TableReader struct {
	Reader
	db    *sql.DB
	table string
	rows  *sql.Rows
}

// FileReader implements Reader reads from the given file
type FileReader struct {
	Reader
	file *os.File
}

// OpenFile opens a file and returns a reader to read from
// the give file on the host.
func OpenFile(fn string) (*FileReader, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, fmt.Errorf("reader failed %s, %v", f.Name(), err)
	}
	r := &FileReader{Reader{nil, nil, 0}, f}
	id := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var f fragment
		f.id = id
		id++
		f.block = strings.Trim(scanner.Text(), "\n")
		r.frams = append(r.frams, f)
	}
	return r, nil
}

// OpenTable returns a reader to read from the given table in db.
func OpenTable(db *sql.DB, table string) (*TableReader, error) {
	has, e := hasTable(db, table)
	if !has {
		return nil, fmt.Errorf("open: table %s doesn't exist", table)
	}
	if e != nil {
		return nil, fmt.Errorf("open: hasTable failed with %v", e)
	}

	r := &TableReader{Reader{nil, nil, 0}, db, table, nil}
	stmt := fmt.Sprintf("SELECT id,block FROM %s;", table)
	r.rows, e = r.db.Query(stmt)
	if e != nil {
		return nil, fmt.Errorf("open: db query [%s] failed: %v", stmt, e)
	}
	// Since statement like `SELECT id,block FROM tbl ORDER BY id` causes an
	// error in hive randomly. We decide to sort results here instead of
	// by SQL engine.
	// Probably you would worry about the dataset is too huge to be holded in
	// memory. It's fine due to we are going to save models to file system
	// for a long term plan.
	for r.rows.Next() {
		var f fragment
		if e = r.rows.Scan(&f.id, &f.block); e != nil {
			r.Close()
			return nil, e
		}
		r.frams = append(r.frams, f)
	}
	sort.Slice(r.frams, func(i, j int) bool {
		return r.frams[i].id < r.frams[j].id
	})
	return r, nil
}

func (r *Reader) Read(p []byte) (n int, e error) {
	n = 0
	for n < len(p) {
		m := copy(p[n:], r.buf)
		n += m
		r.buf = r.buf[m:]
		if len(r.buf) <= 0 {
			if r.cur < len(r.frams) {
				blk := r.frams[r.cur].block
				r.cur++
				if r.buf, e = base64.StdEncoding.DecodeString(blk); e != nil {
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

// Close the reader connection to sqlfs
func (r *TableReader) Close() error {
	if r.rows != nil {
		if e := r.rows.Close(); e != nil {
			return e
		}
		r.rows = nil
	}
	r.db = nil // Mark closed.
	return nil
}

// Close the file handler
func (r *FileReader) Close() error {
	r.file.Close()
	r.file = nil
	return nil
}
