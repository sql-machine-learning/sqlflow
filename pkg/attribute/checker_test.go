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

package attribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFloat32RangeChecker(t *testing.T) {
	a := assert.New(t)

	name := "any_attr_name"

	checker := Float32RangeChecker(0.0, 1.0, true, true)
	a.NoError(checker(1, name))
	a.Error(checker(float32(-1), name))
	a.NoError(checker(float32(0), name))
	a.NoError(checker(float32(0.5), name))
	a.NoError(checker(float32(1), name))
	a.Error(checker(float32(2), name))

	checker2 := Float32RangeChecker(0.0, 1.0, false, false)
	a.NoError(checker(1, name))
	a.Error(checker2(float32(-1), name))
	a.Error(checker2(float32(0), name))
	a.NoError(checker2(float32(0.5), name))
	a.Error(checker2(float32(1), name))
	a.Error(checker2(float32(2), name))
}

func TestIntRangeChecker(t *testing.T) {
	a := assert.New(t)

	name := "any_attr_name"

	checker := IntRangeChecker(0, 2, true, true)
	a.NoError(checker(1.0, name))
	a.Error(checker(int(-1), name))
	a.NoError(checker(int(0), name))
	a.NoError(checker(int(1), name))
	a.NoError(checker(int(2), name))
	a.Error(checker(int(3), name))

	checker2 := IntRangeChecker(0, 2, false, false)
	a.NoError(checker(1.0, name))
	a.Error(checker2(int(-1), name))
	a.Error(checker2(int(0), name))
	a.NoError(checker2(int(1), name))
	a.Error(checker2(int(2), name))
	a.Error(checker2(int(3), name))
}

func TestIntChoicesChecker(t *testing.T) {
	a := assert.New(t)

	name := "any_attr_name"

	checker := IntChoicesChecker(0, 1, 2)
	a.Error(checker(-1, name))
	a.NoError(checker(0, name))
	a.NoError(checker(1, name))
	a.NoError(checker(2, name))
	a.Error(checker(3, name))
}

func TestStringChoicesChecker(t *testing.T) {
	a := assert.New(t)

	name := "any_attr_name"

	checker := StringChoicesChecker("0", "1", "2")
	a.NoError(checker("0", name))
	a.NoError(checker("1", name))
	a.NoError(checker("2", name))
	a.Error(checker("3", name))
}
