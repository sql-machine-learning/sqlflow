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

	"github.com/golang/protobuf/proto"
	pb "sqlflow.org/sqlflow/go/proto"
)

// ProtobufWriter write table as protobuf text format
type ProtobufWriter struct {
	out              io.Writer
	head             map[string]interface{}
	rows             [][]interface{}
	bufSize          int
	hasWrittenHeader bool
}

// NewProtobufWriter returns ProtobufWriter
func createProtobufWriter(bufSize int, out io.Writer) *ProtobufWriter {
	return &ProtobufWriter{
		out:     out,
		head:    make(map[string]interface{}),
		rows:    [][]interface{}{},
		bufSize: bufSize,
	}
}

// SetHeader set the table header
func (table *ProtobufWriter) SetHeader(head map[string]interface{}) error {
	table.head = head
	return nil
}

// AppendRow appends row into buffer
func (table *ProtobufWriter) AppendRow(row []interface{}) error {
	table.rows = append(table.rows, row)
	if len(table.rows) >= table.bufSize {
		if e := table.Flush(); e != nil {
			return e
		}
	}
	return nil
}

// Flush the buffer to writer
func (table *ProtobufWriter) Flush() error {
	if e := table.writeHead(); e != nil {
		return e
	}
	if e := table.writeRows(); e != nil {
		return e
	}
	table.rows = [][]interface{}{}
	return nil
}

// FlushWithError flushes the buffer and end with the error message
func (table *ProtobufWriter) FlushWithError(err error) error {
	if e := table.Flush(); e != nil {
		return nil
	}
	response, e := pb.EncodeMessage(fmt.Sprintf("%v", err))
	if e != nil {
		return e
	}
	return table.formatWrite(response)
}

func (table *ProtobufWriter) writeRows() error {
	for _, row := range table.rows {
		response, e := pb.EncodeRow(row)
		if e != nil {
			return e
		}
		table.formatWrite(response)
	}
	return nil
}

func (table *ProtobufWriter) formatWrite(msg proto.Message) error {
	if e := proto.CompactText(table.out, msg); e != nil {
		return e
	}
	if _, e := table.out.Write([]byte{'\n'}); e != nil {
		return e
	}
	return nil
}

func (table *ProtobufWriter) writeHead() error {
	if len(table.head) == 0 {
		return nil
	}
	// skip write head if it has been written to table.out
	if table.hasWrittenHeader {
		return nil
	}
	response, e := pb.EncodeHead(table.head)
	if e != nil {
		return e
	}
	table.hasWrittenHeader = true
	return table.formatWrite(response)
}
