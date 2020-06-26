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

func literalOrDie(token int, value string) *Expr {
	l, e := NewLiteral(token, value)
	if e != nil {
		panic(e)
	}
	return l
}

func TestSpecialTokenZero(t *testing.T) {
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

	oprd := ExprList{literalOrDie(1, "123")}

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

func TestUnary(t *testing.T) {
	u, e := NewUnary(1, "NOT", nil)
	assert.Nil(t, u)
	assert.Error(t, e) // for the operand is nil.

	u, e = NewUnary('-', "-", literalOrDie(1, "1"))
	assert.NotNil(t, u)
	assert.NoError(t, e)

	assert.False(t, u.IsLiteral())
	assert.False(t, u.IsFuncall())
	assert.False(t, u.IsBinary())
	assert.True(t, u.IsUnary())
	assert.False(t, u.IsVariadic())

	assert.Equal(t, "- 1", u.String())

	u, e = NewUnary(2, "NOT", u)
	assert.NotNil(t, u)
	assert.NoError(t, e) // nested unary expression is OK.

	assert.False(t, u.IsLiteral())
	assert.False(t, u.IsFuncall())
	assert.False(t, u.IsBinary())
	assert.True(t, u.IsUnary())
	assert.False(t, u.IsVariadic())

	assert.Equal(t, "NOT - 1", u.String())
}

func TestBinary(t *testing.T) {
	b, e := NewBinary('+', "+", nil, literalOrDie(1, "2"))
	assert.Nil(t, b)
	assert.Error(t, e)

	b, e = NewBinary('-', "-", literalOrDie(1, "2"), nil)
	assert.Nil(t, b)
	assert.Error(t, e)

	b, e = NewBinary('+', "+", literalOrDie(1, "1"), literalOrDie(1, "2"))
	assert.NotNil(t, b)
	assert.NoError(t, e)

	assert.False(t, b.IsLiteral())
	assert.False(t, b.IsFuncall())
	assert.True(t, b.IsBinary())
	assert.False(t, b.IsUnary())
	assert.False(t, b.IsVariadic())

	assert.Equal(t, "1 + 2", b.String())

	b, e = NewBinary('+', "+", literalOrDie(1, "3"), b)
	assert.NotNil(t, b)
	assert.NoError(t, e)

	assert.False(t, b.IsLiteral())
	assert.False(t, b.IsFuncall())
	assert.True(t, b.IsBinary())
	assert.False(t, b.IsUnary())
	assert.False(t, b.IsVariadic())

	assert.Equal(t, "3 + 1 + 2", b.String())
}

func TestVariadic(t *testing.T) {
	v, e := NewVariadic('[', "{", ExprList{}) // '[' must match "["
	assert.Nil(t, v)
	assert.Error(t, e)

	v, e = NewVariadic('[', "[", ExprList{})
	assert.NotNil(t, v)
	assert.NoError(t, e)

	assert.False(t, v.IsLiteral())
	assert.False(t, v.IsFuncall())
	assert.False(t, v.IsBinary())
	assert.False(t, v.IsUnary())
	assert.True(t, v.IsVariadic())

	assert.Equal(t, "[]", v.String())

	v, e = NewVariadic('[', "[", nil)
	assert.NotNil(t, v)
	assert.NoError(t, e)

	assert.False(t, v.IsLiteral())
	assert.False(t, v.IsFuncall())
	assert.False(t, v.IsBinary())
	assert.False(t, v.IsUnary())
	assert.True(t, v.IsVariadic())

	assert.Equal(t, "[]", v.String())

	v, e = NewVariadic('[', "[", ExprList{literalOrDie(1, "2")})
	assert.NotNil(t, v)
	assert.NoError(t, e)

	assert.False(t, v.IsLiteral())
	assert.False(t, v.IsFuncall())
	assert.False(t, v.IsBinary())
	assert.False(t, v.IsUnary())
	assert.True(t, v.IsVariadic())

	assert.Equal(t, "[2]", v.String())

	v, e = NewVariadic('(', "(", ExprList{v, literalOrDie(1, "1")})
	assert.NotNil(t, v)
	assert.NoError(t, e)

	assert.False(t, v.IsLiteral())
	assert.False(t, v.IsFuncall())
	assert.False(t, v.IsBinary())
	assert.False(t, v.IsUnary())
	assert.True(t, v.IsVariadic())

	assert.Equal(t, "([2], 1)", v.String())

	v, e = NewVariadic('<', "<", ExprList{v, literalOrDie(1, "3")})
	assert.Nil(t, v)
	assert.Error(t, e)
}

func TestFuncall(t *testing.T) {
	v, e := NewFuncall('[', "[", ExprList{}) // any symbol can name function
	assert.NotNil(t, v)
	assert.NoError(t, e)

	assert.False(t, v.IsLiteral())
	assert.True(t, v.IsFuncall())
	assert.False(t, v.IsBinary())
	assert.False(t, v.IsUnary())
	assert.False(t, v.IsVariadic())

	assert.Equal(t, "[()", v.String())

	v, e = NewFuncall('[', "[", nil)
	assert.NotNil(t, v)
	assert.NoError(t, e)

	assert.False(t, v.IsLiteral())
	assert.True(t, v.IsFuncall())
	assert.False(t, v.IsBinary())
	assert.False(t, v.IsUnary())
	assert.False(t, v.IsVariadic())

	assert.Equal(t, "[()", v.String())

	v, e = NewFuncall(1, "sum", ExprList{literalOrDie(1, "2")})
	assert.NotNil(t, v)
	assert.NoError(t, e)

	assert.False(t, v.IsLiteral())
	assert.True(t, v.IsFuncall())
	assert.False(t, v.IsBinary())
	assert.False(t, v.IsUnary())
	assert.False(t, v.IsVariadic())

	assert.Equal(t, "sum(2)", v.String())

	v, e = NewFuncall(1, "print", ExprList{v, literalOrDie(1, "1")})
	assert.NotNil(t, v)
	assert.NoError(t, e)

	assert.False(t, v.IsLiteral())
	assert.True(t, v.IsFuncall())
	assert.False(t, v.IsBinary())
	assert.False(t, v.IsUnary())
	assert.False(t, v.IsVariadic())

	assert.Equal(t, "print(sum(2), 1)", v.String())
}
