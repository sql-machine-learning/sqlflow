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
	pb "sqlflow.org/sqlflow/pkg/proto"
)

// ProtobufTableWriter write table as protobuf text formate
type ProtobufTableWriter struct {
	out             io.Writer
	head            map[string]interface{}
	rows            [][]interface{}
	bufSize         int
	hasWritenHeader bool
}

// NewProtobufTableWriter returns ProtobufTableWriter
func NewProtobufTableWriter(bufSize int, out io.Writer) *ProtobufTableWriter {
	return &ProtobufTableWriter{
		out:     out,
		head:    make(map[string]interface{}),
		rows:    [][]interface{}{},
		bufSize: bufSize,
	}
}

// SetHeader set the table header
func (table *ProtobufTableWriter) SetHeader(head map[string]interface{}) error {
	table.head = head
	return nil
}

// AppendRow appends row into buffer
func (table *ProtobufTableWriter) AppendRow(row []interface{}) error {
	table.rows = append(table.rows, row)
	if len(table.rows) >= table.bufSize {
		if e := table.Flush(); e != nil {
			return e
		}
	}
	return nil
}

// Flush the buffer to writer
func (table *ProtobufTableWriter) Flush() error {
	if e := table.writeHead(); e != nil {
		return e
	}
	if e := table.writeRows(); e != nil {
		return e
	}
	table.rows = [][]interface{}{}
	return nil
}

func (table *ProtobufTableWriter) writeRows() error {
	for _, row := range table.rows {
		response, e := pb.EncodeRow(row)
		if e != nil {
			return e
		}
		return table.formateWrite(response)
	}
	return nil
}

func (table *ProtobufTableWriter) formateWrite(msg proto.Message) error {
	if e := proto.CompactText(table.out, msg); e != nil {
		return e
	}
	if _, e := table.out.Write([]byte{'\n'}); e != nil {
		return e
	}
	return nil
}

func (table *ProtobufTableWriter) writeHead() error {
	if len(table.head) == 0 {
		return fmt.Errorf("should set header")
	}
	// skip write head if it has been writen to table.out
	if table.hasWritenHeader {
		return nil
	}
	response, e := pb.EncodeHead(table.head)
	if e != nil {
		return e
	}
	table.hasWritenHeader = true
	return table.formateWrite(response)
}
