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
// limitations under the License.

package ast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPanicWithType0(t *testing.T) {
	var (
		expr *Expr
		e    error
	)

	expr, e = NewLiteral(0, "")
	assert.Nil(t, expr)
	assert.Error(t, e)

	expr, e = NewLiteral(0, "something")
	assert.Nil(t, expr)
	assert.Error(t, e)

	expr, e = NewLiteral(1, "123")
	assert.NotNil(t, expr)
	assert.NoError(t, e)
	oprd := ExprList{expr}

	expr, e = NewFuncall(0, "something", oprd)
	assert.Nil(t, expr)
	assert.Error(t, e)

	expr, e = NewUnary(0, "something", oprd[0])
	assert.Nil(t, expr)
	assert.Error(t, e)

	expr, e = NewVariadic(0, "something", oprd)
	assert.Nil(t, expr)
	assert.Error(t, e)

	expr, e = NewFuncall(0, "something", oprd)
	assert.Nil(t, expr)
	assert.Error(t, e)
}
