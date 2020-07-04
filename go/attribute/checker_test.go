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

	checker := Float32RangeChecker(0.0, 1.0, true, true)
	a.NoError(checker(1))
	a.Error(checker(float32(-1)))
	a.NoError(checker(float32(0)))
	a.NoError(checker(float32(0.5)))
	a.NoError(checker(float32(1)))
	a.Error(checker(float32(2)))

	checker2 := Float32RangeChecker(0.0, 1.0, false, false)
	a.NoError(checker(1))
	a.Error(checker2(float32(-1)))
	a.Error(checker2(float32(0)))
	a.NoError(checker2(float32(0.5)))
	a.Error(checker2(float32(1)))
	a.Error(checker2(float32(2)))
}

func TestIntRangeChecker(t *testing.T) {
	a := assert.New(t)

	checker := IntRangeChecker(0, 2, true, true)
	a.NoError(checker(1.0))
	a.Error(checker(int(-1)))
	a.NoError(checker(int(0)))
	a.NoError(checker(int(1)))
	a.NoError(checker(int(2)))
	a.Error(checker(int(3)))

	checker2 := IntRangeChecker(0, 2, false, false)
	a.NoError(checker(1.0))
	a.Error(checker2(int(-1)))
	a.Error(checker2(int(0)))
	a.NoError(checker2(int(1)))
	a.Error(checker2(int(2)))
	a.Error(checker2(int(3)))
}

func TestIntChoicesChecker(t *testing.T) {
	a := assert.New(t)

	checker := IntChoicesChecker(0, 1, 2)
	a.Error(checker(-1))
	a.NoError(checker(0))
	a.NoError(checker(1))
	a.NoError(checker(2))
	a.Error(checker(3))
}

func TestStringChoicesChecker(t *testing.T) {
	a := assert.New(t)

	checker := StringChoicesChecker("0", "1", "2")
	a.NoError(checker("0"))
	a.NoError(checker("1"))
	a.NoError(checker("2"))
	a.Error(checker("3"))
}
