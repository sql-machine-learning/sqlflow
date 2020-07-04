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
	"bufio"
	"bytes"
	"io"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/stretchr/testify/assert"
	pb "sqlflow.org/sqlflow/go/proto"
)

func TestProtobufWriter(t *testing.T) {
	a := assert.New(t)
	b := new(bytes.Buffer)
	table, e := Create("protobuf", 1, b)
	a.NoError(e)
	table.SetHeader(mockHead())
	rows := mockRows()
	for _, row := range rows {
		table.AppendRow(row)
	}
	table.Flush()

	reader := bufio.NewReader(b)

	// check head
	response := &pb.Response{}
	head, e := reader.ReadString('\n')
	e = proto.UnmarshalText(head, response)
	a.NoError(e)
	a.Equal(mockHead()["columnNames"].([]string), response.GetHead().GetColumnNames())

	// check rows
	for _, row := range rows {
		line, e := reader.ReadString('\n')
		if e != nil {
			if e == io.EOF {
				break
			}
			a.NoError(e)
		}
		e = proto.UnmarshalText(line, response)
		a.NoError(e)
		for i, element := range row {
			expectedValue := &wrappers.DoubleValue{Value: element.(float64)}
			pm, e := ptypes.MarshalAny(expectedValue)
			a.NoError(e)
			a.Equal(pm, response.GetRow().Data[i])
		}
	}
}

func TestEmptyProtobufWriter(t *testing.T) {
	a := assert.New(t)
	b := new(bytes.Buffer)
	table, e := Create("protobuf", 1, b)
	a.NoError(e)
	// no output if step execute an extended sql
	a.NoError(table.Flush())
}
