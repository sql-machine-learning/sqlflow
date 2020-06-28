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

func TestAttributesUnion(t *testing.T) {
	a := Attributes{}

	b, e := a.Union(nil)
	assert.Equal(t, 0, len(b))
	assert.NoError(t, e)

	b, e = a.Union(Attributes{})
	assert.Equal(t, 0, len(b))
	assert.NoError(t, e)

	a = Attributes{"apple": literalOrDie(1, "123")}
	b, e = a.Union(Attributes{"apple": literalOrDie(1, "123")})
	assert.Nil(t, b)
	assert.Error(t, e)
}
