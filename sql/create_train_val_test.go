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

func TestCreateTrainAndValDataset(t *testing.T) {
	a := assert.New(t)
	_, e := newTrainAndValDataset(testDB, testTrainAndValDataset, "orig", 1)
	a.Error(e)
	_, e = newTrainAndValDataset(testDB, testTrainAndValDataset, "orig", 0)
	a.Error(e)

	ds, e := newTrainAndValDataset(testDB, testTrainAndValDataset, "orig", 0.8)
	a.NoError(e)
	if testDB.driverName == "maxcompute" {
		a.True(ds.supported)
	}
}

// TODO(weiguo): add test cases for hive&maxcompute
