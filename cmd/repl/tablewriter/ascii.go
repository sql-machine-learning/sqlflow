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

// TableWriter write the Table a speicial format,
// the example code of ASCII formater:
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
}

// NewTableWriter a TableWriter instance
func NewTableWriter(name string, bufSize int, w io.Writer) (TableWriter, error) {
	if name == "ascii" {
		return newASCIITableWriter(bufSize, w), nil
	} else if name == "protobuf" {
		return NewProtobufTableWriter(bufSize, w), nil
	}
	return nil, fmt.Errorf("SQLFLow does not support the tablewriter : %s", name)
}

// ASCIITableWriter write table as ASCII formate
type ASCIITableWriter struct {
	table   *tablewriter.Table
	bufSize int
}

func newASCIITableWriter(bufSize int, w io.Writer) *ASCIITableWriter {
	return &ASCIITableWriter{
		table:   tablewriter.NewWriter(w),
		bufSize: bufSize,
	}
}

// SetHeader set the table header
func (t *ASCIITableWriter) SetHeader(head map[string]interface{}) error {
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
func (t *ASCIITableWriter) AppendRow(row []interface{}) error {
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

// Flush the buffer
func (t *ASCIITableWriter) Flush() error {
	if t.table.NumLines() > 0 {
		t.table.Render()
		t.table.ClearRows()
	}
	return nil
}
