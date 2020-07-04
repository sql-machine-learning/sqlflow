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
// limitations under the License.o

package xgboost

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/go/ir"
)

func TestExplain(t *testing.T) {
	a := assert.New(t)
	tir := ir.MockTrainStmt(true)
	astmt := &ir.ExplainStmt{
		Select:    "SELECT * FROM iris.train",
		Explainer: "TreeExplainer",
		Attributes: map[string]interface{}{
			"summary.plot_type": "dot",
			"others.type":       "bar",
		},
		TrainStmt: tir,
	}
	_, err := Explain(astmt, mockSession())
	a.NoError(err)
}
