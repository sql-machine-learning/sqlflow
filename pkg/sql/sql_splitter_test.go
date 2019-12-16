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

package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitMultipleSQL(t *testing.T) {
	a := assert.New(t)
	splitted, err := SplitMultipleSQL(`CREATE TABLE copy_table_1 AS SELECT a,b,c FROM table_1 WHERE c<>";";
SELECT * FROM copy_table_1;SELECT * FROM copy_table_1 TO TRAIN DNNClassifier WITH n_classes=2 INTO test_model;`)
	a.NoError(err)
	a.Equal("CREATE TABLE copy_table_1 AS SELECT a,b,c FROM table_1 WHERE c<>\";\";", splitted[0])
	a.Equal("SELECT * FROM copy_table_1;", splitted[1])
	a.Equal("SELECT * FROM copy_table_1 TO TRAIN DNNClassifier WITH n_classes=2 INTO test_model;", splitted[2])
}
