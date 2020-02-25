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
	"testing"

	"github.com/stretchr/testify/assert"
)

var expectedTableASCII = `+------+------+
| COL1 | COL2 |
+------+------+
|  1.0 |  1.1 |
|  2.0 |  2.1 |
+------+------+
`

func TestASCIIWriter(t *testing.T) {
	a := assert.New(t)
	b := new(bytes.Buffer)
	table, e := NewTableWriter("ascii", 1000, b)
	table.SetHeader([]string{"col1", "col2"})
	table.AppendRow([]string{"1.0", "1.1"})
	table.AppendRow([]string{"2.0", "2.1"})
	a.NoError(table.Flush())
	a.NoError(e)
	a.Equal(b.String(), expectedTableASCII)
}
