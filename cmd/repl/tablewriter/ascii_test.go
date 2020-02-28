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
	"bytes"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TODO(yancey1989): interfaceSlice function would convert 1.0 to 1
// better to keep the accuracy.
var expectedTableASCII = `+------+------+
| COL1 | COL2 |
+------+------+
|    1 |  1.1 |
|    2 |  2.1 |
+------+------+
`

func mockHead() map[string]interface{} {
	head := make(map[string]interface{})
	cols := []string{"col1", "col2"}
	head["columnNames"] = cols
	return head
}

func interfaceSlice(slice interface{}) []interface{} {
	s := reflect.ValueOf(slice)
	ret := make([]interface{}, s.Len())
	for i := 0; i < s.Len(); i++ {
		ret[i] = s.Index(i).Interface()
	}
	return ret
}

func mockRows() [][]interface{} {
	rows := [][]interface{}{}
	rows = append(rows, interfaceSlice([]float64{1.0, 1.1}))
	rows = append(rows, interfaceSlice([]float64{2.0, 2.1}))
	return rows
}

func TestASCIIWriter(t *testing.T) {
	a := assert.New(t)
	b := new(bytes.Buffer)
	table, e := Create("ascii", 1000, b)
	a.NoError(table.SetHeader(mockHead()))
	for _, row := range mockRows() {
		table.AppendRow(row)
	}
	a.NoError(table.Flush())
	a.NoError(e)
	a.Equal(expectedTableASCII, b.String())
}

func TestEmptyASCIIWriter(t *testing.T) {
	a := assert.New(t)
	b := new(bytes.Buffer)
	table, e := Create("ascii", 1000, b)
	a.NoError(e)
	a.NoError(table.Flush())
}
