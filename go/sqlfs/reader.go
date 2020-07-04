// Copyright 2020 The SQLFlow Authors. All rights reserved.
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
	"sort"
)

type fragment struct {
	id    int
	block string
}

// Reader implements io.ReadCloser
type Reader struct {
	db    *sql.DB
	table string
	buf   []byte
	rows  *sql.Rows
	frams []fragment
	cur   int
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

	r := &Reader{db, table, nil, nil, nil, 0}
	stmt := fmt.Sprintf("SELECT id,block FROM %s;", table)
	r.rows, e = r.db.Query(stmt)
	if e != nil {
		return nil, fmt.Errorf("open: db query [%s] failed: %v", stmt, e)
	}
	// Since statement like `SELECT id,block FROM tbl ORDER BY id` causes an
	// error in hive randomly. We decide to sort results here instead of
	// by SQL engine.
	// Probably you would worry about the dataset is too huge to be held in
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
	if r.db == nil {
		return 0, fmt.Errorf("read from a closed reader")
	}
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
