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
	SetHeader([]string) error
	AppendRow([]string) error
	Flush() error
}

// NewTableWriter a TableWriter instance
func NewTableWriter(formater string, bufSize int, w io.Writer) (TableWriter, error) {
	if formater == "ascii" {
		return newASCIITableWriter(bufSize, w), nil
	}
	return nil, fmt.Errorf("SQLFLow does not support the table formater: %s", formater)
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
func (s *ASCIITableWriter) SetHeader(header []string) error {
	if len(header) == 0 {
		return fmt.Errorf("header columns should not be empty")
	}
	s.table.SetHeader(header)
	return nil
}

// AppendRow append row data
func (s *ASCIITableWriter) AppendRow(rows []string) error {
	s.table.Append(rows)
	if s.table.NumLines() >= s.bufSize {
		s.table.Render()
		s.table.ClearRows()
	}
	return nil
}

// Flush the buffer
func (s *ASCIITableWriter) Flush() error {
	if s.table.NumLines() > 0 {
		s.table.Render()
		s.table.ClearRows()
	}
	return nil
}
