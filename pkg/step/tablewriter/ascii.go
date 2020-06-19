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

package tablewriter

import (
	"fmt"
	"io"

	"github.com/olekukonko/tablewriter"
)

// TableWriter write the Table a special format,
// the example code of ASCII formatter:
//
// table := NewTableWriter("ascii", 1024, os.Stdout)
// defer table.Flush()
// table.SetHeader("col1", "col2")
// table.AppendRow([]string{"1.0", "1.1"})
// table.AppendRow([]string{"2.0", "2.1"})
// if e := table.Flush(); e != nil {
//   log.Falt(e)
// }
//
type TableWriter interface {
	SetHeader(map[string]interface{}) error
	AppendRow([]interface{}) error
	Flush() error
	FlushWithError(error) error
}

// Create returns a TableWriter instance
func Create(name string, bufSize int, w io.Writer) (TableWriter, error) {
	if name == "ascii" {
		return createASCIIWriter(bufSize, w), nil
	} else if name == "protobuf" {
		return createProtobufWriter(bufSize, w), nil
	}
	return nil, fmt.Errorf("SQLFLow does not support the tablewriter : %s", name)
}

// ASCIIWriter write table as ASCII format
type ASCIIWriter struct {
	table   *tablewriter.Table
	bufSize int
	w       io.Writer
}

func createASCIIWriter(bufSize int, w io.Writer) *ASCIIWriter {
	return &ASCIIWriter{
		table:   tablewriter.NewWriter(w),
		bufSize: bufSize,
		w:       w,
	}
}

// SetHeader set the table header
func (t *ASCIIWriter) SetHeader(head map[string]interface{}) error {
	cn, ok := head["columnNames"]
	if !ok {
		return fmt.Errorf("can't find field columnNames in head")
	}
	cols, ok := cn.([]string)
	if !ok {
		return fmt.Errorf("invalid header type")
	}
	t.table.SetHeader(cols)
	return nil
}

// AppendRow append row data
func (t *ASCIIWriter) AppendRow(row []interface{}) error {
	s := []string{}
	for _, d := range row {
		s = append(s, fmt.Sprint(d))
	}
	t.table.Append(s)
	if t.table.NumLines() >= t.bufSize {
		t.table.Render()
		t.table.ClearRows()
	}
	return nil
}

// FlushWithError flushes the buffer and end with the error message
func (t *ASCIIWriter) FlushWithError(e error) error {
	if e := t.Flush(); e != nil {
		return e
	}
	_, e = t.w.Write([]byte(fmt.Sprintf("%v", e)))
	return e
}

// Flush the buffer
func (t *ASCIIWriter) Flush() error {
	if t.table.NumLines() > 0 {
		t.table.Render()
		t.table.ClearRows()
	}
	return nil
}
