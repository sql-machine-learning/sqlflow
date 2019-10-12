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

package attribute

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDictionary_Validate(t *testing.T) {
	a := assert.New(t)

	checker := func(i interface{}) error {
		ii, ok := i.(int)
		if !ok {
			return fmt.Errorf("%T %v should of type integer", i, i)
		}
		if ii < 0 {
			return fmt.Errorf("some error")
		}
		return nil
	}
	tb := Dictionary{"a": {Int, "attribute a", checker}}
	a.NoError(tb.Validate(map[string]interface{}{"a": 1}))
	a.EqualError(tb.Validate(map[string]interface{}{"a": -1}), "some error")
	a.EqualError(tb.Validate(map[string]interface{}{"_a": -1}), fmt.Sprintf(errUnsupportedAttribute, "_a"))
	a.EqualError(tb.Validate(map[string]interface{}{"a": 1.0}), fmt.Sprintf(errUnexpectedType, "a", "Int", 1.0))
}
