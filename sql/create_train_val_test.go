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

const (
	testTrainAndValDataset = `
SELECT a.sepal_length,b.sepal_width,a.petal_length,b.petal_width,a.class
FROM iris_train a,iris_test b
WHERE a.class=b.class
LIMIT 7
`
	testHiveTrainAndValDataset = `
SELECT a.sepal_length,b.sepal_width,a.petal_length,b.petal_width,a.class
FROM iris.train a,iris.test b
WHERE a.class=b.class
LIMIT 7
`
)

func TestCreateTrainAndValDataset(t *testing.T) {
	a := assert.New(t)

	switch testDB.driverName {
	case "maxcompute":
		_, e := newTrainAndValDataset(testDB, testTrainAndValDataset, "orig", 1)
		a.Error(e)
		_, e = newTrainAndValDataset(testDB, testTrainAndValDataset, "orig", 0)
		a.Error(e)
		ds, e := newTrainAndValDataset(testDB, testTrainAndValDataset, "orig", 0.8)
		a.NoError(e)
		a.True(ds.supported)
	case "hive":
		_, e := newTrainAndValDataset(testDB, testHiveTrainAndValDataset, "orig", 1)
		a.Error(e)
		_, e = newTrainAndValDataset(testDB, testHiveTrainAndValDataset, "orig", 0)
		a.Error(e)
		ds, e := newTrainAndValDataset(testDB, testHiveTrainAndValDataset, "orig", 0.8)
		a.NoError(e)
		a.True(ds.supported)
	}
}
