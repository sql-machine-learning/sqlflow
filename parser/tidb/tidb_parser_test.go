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
	var (
		i int
		e error
	)

	Init()

	_, i, e = ParseAndSplit("SELECTED a FROM t1") // SELECTED => SELECT
	a.Equal(-1, i)
	a.Error(e)

	_, i, e = ParseAndSplit("SELECT * FROM t1 TO TRAIN DNNClassifier")
	a.Equal(17, i)
	a.NoError(e)

	_, i, e = ParseAndSplit("SELECT * FROM t1 TO TO TRAIN DNNClassifier")
	a.Equal(17, i)
	a.NoError(e)

	_, i, e = ParseAndSplit("SELECT * FROM t1 t2 TO TRAIN DNNClassifier") // t2 is an alias of t1
	a.Equal(20, i)
	a.NoError(e)

	_, i, e = ParseAndSplit("SELECT * FROM t1 t2, t3 TO TRAIN DNNClassifier") // t2 is an alias of t1
	a.Equal(24, i)
	a.NoError(e)

	_, i, e = ParseAndSplit("SELECT * FROM t1 t2, t3 t4 TO TRAIN DNNClassifier") // t2 and t4 are aliases.
	a.Equal(27, i)
	a.NoError(e)

	_, i, e = ParseAndSplit("SELECT * FROM (SELECT * FROM t1)")
	a.Equal(-1, i)
	a.Error(e) // TiDB parser and MySQL require an alias name after the nested SELECT.

	_, i, e = ParseAndSplit("SELECT * FROM (SELECT * FROM t1) t2")
	a.Equal(-1, i)
	a.NoError(e)

	_, i, e = ParseAndSplit("SELECT * FROM (SELECT * FROM t1) t2 TO TRAIN DNNClassifier")
	a.Equal(36, i)
	a.NoError(e)
}
