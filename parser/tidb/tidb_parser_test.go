// Copyright 2019 The SQLFlow Authors. All rights reserved.
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

package tidb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTiDBParser(t *testing.T) {
	a := assert.New(t)

	i, e := Parse("select * frm tbl") // frm->from
	a.NoError(e)
	a.Equal(9, i)

	i, e = Parse("select * from tbl where id = 1")
	a.NoError(e)
	a.Equal(-1, i)

	i, e = Parse("select * from tbl where id = 1 TRAIN DNNClassifier WITH learning_rate=0.1")
	a.NoError(e)
	a.Equal(31, i)

	i, e = Parse("select * from tbl where id = 1 predict tbl.predicted using my_model")
	a.NoError(e)
	a.Equal(31, i)

	i, e = Parse("select * from tbl where id = 1 predict tbl.predicted uses my_model") // uses->using
	a.NoError(e)
	a.Equal(31, i)
}
