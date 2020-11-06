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

func readNextBlocks(db *sql.DB, table string, startRowIdx, rowBufSize int) ([]*fragment, error) {
	stmt := fmt.Sprintf("SELECT id,block FROM %s WHERE id>=%d AND id<%d;", table, startRowIdx, startRowIdx+rowBufSize)
	rows, err := db.Query(stmt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fragments []*fragment
	for rows.Next() {
		f := &fragment{}
		if err := rows.Scan(&f.id, &f.block); err != nil {
			return nil, err
		}
		fragments = append(fragments, f)
	}

	if len(fragments) > rowBufSize {
		return nil, fmt.Errorf("invalid sqlfs db table %s", table)
	}

	sort.Slice(fragments, func(i, j int) bool {
		return fragments[i].id < fragments[j].id
	})

	for i, f := range fragments {
		if f.id != i+startRowIdx {
			return nil, fmt.Errorf("invalid sqlfs db table %s", table)
		}
	}
	return fragments, nil
}

// reader implements io.ReadCloser
type reader struct {
	db          *sql.DB
	table       string
	buf         []byte
	fragments   []*fragment
	fragmentIdx int
	rowIdx      int
	rowBufSize  int
}

func (r *reader) nextBlock() (string, error) {
	var err error
	if r.fragmentIdx == len(r.fragments) {
		if r.rowIdx > 0 && len(r.fragments) < r.rowBufSize {
			return "", io.EOF
		}

		r.fragments, err = readNextBlocks(r.db, r.table, r.rowIdx, r.rowBufSize)
		if err != nil {
			return "", err
		}
		r.rowIdx += len(r.fragments)
		r.fragmentIdx = 0
	}

	if len(r.fragments) == 0 {
		return "", io.EOF
	}
	block := r.fragments[r.fragmentIdx].block
	r.fragmentIdx++
	return block, nil
}

// Open returns a reader to read from the given table in db.
func Open(db *sql.DB, table string, rowBufSize int) (io.ReadCloser, error) {
	has, e := hasTable(db, table)
	if !has {
		return nil, fmt.Errorf("open: table %s doesn't exist", table)
	}
	if e != nil {
		return nil, fmt.Errorf("open: hasTable failed with %v", e)
	}
	if rowBufSize <= 0 {
		return nil, fmt.Errorf("rowBufSize must be larger than 0")
	}

	return &reader{
		db:          db,
		table:       table,
		buf:         nil,
		fragments:   nil,
		fragmentIdx: 0,
		rowIdx:      0,
		rowBufSize:  rowBufSize,
	}, nil
}

func (r *reader) Read(p []byte) (n int, e error) {
	if r.db == nil {
		return 0, fmt.Errorf("read from a closed reader")
	}

	n = 0
	for n < len(p) {
		m := copy(p[n:], r.buf)
		n += m
		r.buf = r.buf[m:]
		if len(r.buf) <= 0 {
			var blk string
			blk, e = r.nextBlock()
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
func (r *reader) Close() error {
	r.db = nil // Mark closed.
	return nil
}
