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
	a.Error(e)

	i, e = Parse("SELECT a FROM t1") // (i,e)==(-1,nil) indicates legal native SQL syntax.
	a.Equal(-1, i)
	a.NoError(e)

	i, e = Parse("SELECT a, b FROM t1")
	a.Equal(-1, i)
	a.NoError(e)

	i, e = Parse("SELECT a b FROM t1") // Calcite doesn't need ',' between fields.
	a.Equal(-1, i)
	a.NoError(e)

	i, e = Parse("SELECT a b c FROM t1") // Calcite doesn't accept three fields without ','.
	a.Equal(11, i)
	a.NoError(e)

	i, e = Parse("SELECT * FROM t1")
	a.Equal(-1, i)
	a.NoError(e)

	i, e = Parse("SELECT * FROM t1, t2")
	a.Equal(-1, i)
	a.NoError(e)

	i, e = Parse("SELECT * FROM t1 t2") // Calcite doesn't need ',' between two tables.
	a.Equal(-1, i)
	a.NoError(e)

	i, e = Parse("SELECT * FROM t1 t2 t3") // Calcite doesn't accept three fields without ','.
	a.Equal(20, i)
	a.NoError(e)

	i, e = Parse("SELECT a FROM t1 WHERE a IN (SELECT a FROM t2 WHERE Quantity > 100)")
	a.Equal(-1, i)
	a.NoError(e)

	i, e = Parse("SELECT a FROM t1 WHERE a IN (SELECT a FROM t2 WHERE Quantity > 100) TRAIN DNNClassifier")
	a.Equal(68, i) // before TRAIN
	a.NoError(e)

	i, e = Parse("SELECT a FROM t1 WHERE a IN (SELECT a FROM t2 WHERE Quantity > 100) Predict DNNClassifier")
	a.Equal(68, i) // before Predict
	a.NoError(e)

	i, e = Parse("SELECT a FROM t1 PREDICT DNNClassifier")
	a.Equal(25, i) // This looks a bug of Calcite parser. See cases ahead.
	a.NoError(e)
}
