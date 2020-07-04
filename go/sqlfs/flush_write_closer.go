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

// flushWriteCloser implements io.WriteCloser with two hooks: (1)
// flush, which is supposed to be called by Write when the internal
// buffer overflows, and (2) wrapup, which is to be called by Close.
// We need flushWriteCloser to implement the SQL writer and the Hive
// writer.  For more details, please read sql_writer.go and
// hive_writer.go.
type flushWriteCloser struct {
	buf     []byte
	flushes int // record the count of flushes.
	flush   func([]byte) error
	wrapup  func() error
}

func newFlushWriteCloser(flush func([]byte) error, wrapup func() error, flushCap int) *flushWriteCloser {
	return &flushWriteCloser{
		buf:     make([]byte, 0, flushCap),
		flushes: 0,
		flush:   flush,
		wrapup:  wrapup}
}

// Accumulate p into the buffer.  Flush if overflow.
func (w *flushWriteCloser) Write(p []byte) (n int, e error) {
	n = 0
	for len(p) > 0 {
		fill := cap(w.buf) - len(w.buf)
		if fill > len(p) {
			fill = len(p)
		}
		w.buf = append(w.buf, p[:fill]...)
		p = p[fill:]
		n += fill
		if len(w.buf) >= cap(w.buf) {
			if e = w.flush(w.buf); e != nil {
				return n, e
			}
			w.buf = w.buf[:0]
		}
	}
	return n, nil
}

func (w *flushWriteCloser) Close() error {
	if e := w.flush(w.buf); e != nil {
		return e
	}
	w.buf = w.buf[:0]
	if e := w.wrapup(); e != nil {
		return e
	}
	return nil
}
