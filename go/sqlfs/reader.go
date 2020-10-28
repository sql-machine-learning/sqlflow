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

type blockReader interface {
	nextBlock() (string, error)
}

type readAllOnceBlockReader struct {
	frames []*fragment
	cur    int
}

func newReadAllOnceBlockReader(db *sql.DB, table string) (blockReader, error) {
	r := &readAllOnceBlockReader{
		frames: nil,
		cur:    0,
	}

	stmt := fmt.Sprintf("SELECT id,block FROM %s;", table)
	rows, e := db.Query(stmt)
	if e != nil {
		return nil, fmt.Errorf("open: db query [%s] failed: %v", stmt, e)
	}
	defer rows.Close()

	// Since statement like `SELECT id,block FROM tbl ORDER BY id` causes an
	// error in hive randomly. We decide to sort results here instead of
	// by SQL engine.
	// Probably you would worry about the dataset is too huge to be held in
	// memory. It's fine due to we are going to save models to file system
	// for a long term plan.
	for rows.Next() {
		f := &fragment{}
		if e = rows.Scan(&f.id, &f.block); e != nil {
			return nil, e
		}
		r.frames = append(r.frames, f)
	}
	sort.Slice(r.frames, func(i, j int) bool {
		return r.frames[i].id < r.frames[j].id
	})
	return r, nil
}

func (r *readAllOnceBlockReader) nextBlock() (string, error) {
	if r.cur < len(r.frames) {
		f := r.frames[r.cur]
		r.cur++
		return f.block, nil
	}
	return "", io.EOF
}

type readOneByOneBlockReader struct {
	db    *sql.DB
	table string
	cur   int
}

func (r *readOneByOneBlockReader) nextBlock() (string, error) {
	stmt := fmt.Sprintf("SELECT id,block FROM %s WHERE id=%d;", r.table, r.cur)
	rows, e := r.db.Query(stmt)
	if e != nil {
		return "", e
	}
	defer rows.Close()

	if !rows.Next() {
		return "", io.EOF
	}

	f := &fragment{}
	if e = rows.Scan(&f.id, &f.block); e != nil {
		return "", e
	}

	if rows.Next() {
		return "", fmt.Errorf("invalid sqlfs db: duplicate id %d", r.cur)
	}

	r.cur++
	return f.block, nil
}

// Reader implements io.ReadCloser
type Reader struct {
	buf []byte
	br  blockReader
}

// Open returns a reader to read from the given table in db.
func Open(db *sql.DB, table string, readAllOnce bool) (*Reader, error) {
	has, e := hasTable(db, table)
	if !has {
		return nil, fmt.Errorf("open: table %s doesn't exist", table)
	}
	if e != nil {
		return nil, fmt.Errorf("open: hasTable failed with %v", e)
	}

	var br blockReader
	if readAllOnce {
		br, e = newReadAllOnceBlockReader(db, table)
		if e != nil {
			return nil, e
		}
	} else {
		br = &readOneByOneBlockReader{
			db:    db,
			table: table,
			cur:   0,
		}
	}

	return &Reader{buf: nil, br: br}, nil
}

func (r *Reader) Read(p []byte) (n int, e error) {
	if r.br == nil {
		return 0, fmt.Errorf("read from a closed reader")
	}

	n = 0
	for n < len(p) {
		m := copy(p[n:], r.buf)
		n += m
		r.buf = r.buf[m:]
		if len(r.buf) <= 0 {
			var blk string
			blk, e = r.br.nextBlock()
			if e != nil {
				break
			}

			if r.buf, e = base64.StdEncoding.DecodeString(blk); e != nil {
				break
			}
		}
	}
	return n, e
}

// Close the reader connection to sqlfs
func (r *Reader) Close() error {
	r.br = nil // Mark closed.
	return nil
}
