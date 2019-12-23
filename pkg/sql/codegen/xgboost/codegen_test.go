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

package xgboost

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

func TestAttributes(t *testing.T) {
	a := assert.New(t)
	a.Equal(27, len(attributeDictionary))
}

func TestTrainAndPredict(t *testing.T) {
	a := assert.New(t)
	tir := ir.MockTrainStmt(database.MockURL(), true)
	_, err := Train(tir)
	a.NoError(err)

	pir := ir.MockPredStmt(tir)
	code, err := Pred(pir, database.MockSession())

	r, _ := regexp.Compile(`hdfs_user='''(.*)'''`)
	a.Equal(r.FindStringSubmatch(code)[1], "sqlflow_admin")
	r, _ = regexp.Compile(`hdfs_pass='''(.*)'''`)
	a.Equal(r.FindStringSubmatch(code)[1], "sqlflow_pass")

	a.NoError(err)
}
