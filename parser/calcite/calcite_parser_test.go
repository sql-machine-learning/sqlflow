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

package calcite

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalciteParser(t *testing.T) {
	if addr := os.Getenv("SQLFLOW_CALCITE_PARSER"); len(addr) > 0 {
		Init(addr)
		defer Cleanup()
	} else {
		t.Logf("Cannot connect to CalciteParserServer; skip TestCalciteParser")
		return
	}

	var (
		i int
		e error
		a = assert.New(t)
	)

	i, e = Parse("SELECTED a FROM t1") // SELECTED => SELECT
	a.Equal(0, i)
	a.Error(e) // The second parse is on "", for which, Calcite parser errs.

	i, e = Parse("SELECT * FROM t1 TO TRAIN DNNClassifier")
	a.Equal(17, i)
	a.NoError(e)

	i, e = Parse("SELECT * FROM t1 TO TO TRAIN DNNClassifier")
	a.Equal(17, i)
	a.NoError(e)

	i, e = Parse("SELECT * FROM t1 t2 TO TRAIN DNNClassifier") // t2 is an alias of t1
	a.Equal(20, i)
	a.NoError(e)

	i, e = Parse("SELECT * FROM t1 t2, t3 TO TRAIN DNNClassifier") // t2 is an alias of t1
	a.Equal(24, i)
	a.NoError(e)

	i, e = Parse("SELECT * FROM t1 t2, t3 t4 TO TRAIN DNNClassifier") // t2 and t4 are aliases.
	a.Equal(27, i)
	a.NoError(e)

	i, e = Parse("SELECT * FROM (SELECT * FROM t1)")
	a.Equal(-1, i)
	a.NoError(e)

	i, e = Parse("SELECT * FROM (SELECT * FROM t1) TO TRAIN DNNClassifier")
	a.Equal(33, i)
	a.NoError(e)
}
