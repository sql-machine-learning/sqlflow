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

	"github.com/sql-machine-learning/sqlflow/sql/codegen"

	"github.com/stretchr/testify/assert"
)

func TestGenerateTrainIR(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	normal := `SELECT c1, c2, c3,c4
FROM my_table
TRAIN DNNClassifier
WITH model.n_classes=2, train.optimizer="adam"
COLUMN c1,NUMERIC(c2, [128, 32]),CATEGORY_ID(c3, 512)
LABEL c4
INTO mymodel;`

	r, e := parser.Parse(normal)
	a.NoError(e)

	trainIR, err := generateTrainIR(r, "mysql://somestring")
	a.NoError(err)
	a.Equal("DNNClassifier", trainIR.Estimator)
	a.Equal("SELECT c1, c2, c3, c4\nFROM my_table", trainIR.Select)

	a.Equal("model.n_classes", trainIR.Attributes[0].Key)
	a.Equal("2", trainIR.Attributes[0].Value)
	a.Equal("train.optimizer", trainIR.Attributes[1].Key)
	a.Equal("\"adam\"", trainIR.Attributes[1].Value)

	nc, ok := trainIR.Features["feature_columns"][0].(*codegen.NumericColumn)
	a.True(ok)
	a.Equal([]int{1}, nc.FieldMeta.Shape)

	nc, ok = trainIR.Features["feature_columns"][1].(*codegen.NumericColumn)
	a.True(ok)
	a.Equal("c2", nc.FieldMeta.Name)
	a.Equal([]int{128, 32}, nc.FieldMeta.Shape)

	cc, ok := trainIR.Features["feature_columns"][2].(*codegen.CategoryIDColumn)
	a.True(ok)
	a.Equal("c3", cc.FieldMeta.Name)
	a.Equal(512, cc.BucketSize)

	l, ok := trainIR.Label.(*codegen.LabelColumn)
	a.True(ok)
	a.Equal("c4", l.FieldMeta.Name)
}
