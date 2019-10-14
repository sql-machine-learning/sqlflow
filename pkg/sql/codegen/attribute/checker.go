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
)

func newFloat32(f float32) *float32 {
	return &f
}

// Float32RangeChecker is a helper function to generate range checkers on attribute.
// lower/upper indicates the lower bound and upper bound of the attribute value.
// If lower/upper is nil, it means no boundary.
// includeLower/includeUpper indicates the inclusion of the bound.
func Float32RangeChecker(lower, upper *float32, includeLower, includeUpper bool) func(interface{}) error {

	checker := func(e interface{}) error {
		f, ok := e.(float32)
		if !ok {
			return fmt.Errorf("expected type float32, received %T", e)
		}

		// NOTE(tony): nil means no boundary
		if lower != nil {
			if includeLower && !(*lower <= f) {
				return fmt.Errorf("range check %v <= %v failed", *lower, f)
			}
			if !includeLower && !(*lower < f) {
				return fmt.Errorf("range check %v < %v failed", *lower, f)
			}
		}

		// NOTE(tony): nil means no boundary
		if upper != nil {
			if includeUpper && !(f <= *upper) {
				return fmt.Errorf("range check %v <= %v failed", f, *upper)
			}
			if !includeUpper && !(f < *upper) {
				return fmt.Errorf("range check %v < %v failed", f, *upper)
			}
		}

		return nil
	}

	return checker
}

func newInt(i int) *int {
	return &i
}

// IntRangeChecker is a helper function to generate range checkers on attribute.
// lower/upper indicates the lower bound and upper bound of the attribute value.
// If lower/upper is nil, it means no boundary.
// includeLower/includeUpper indicates the inclusion of the bound.
func IntRangeChecker(lower, upper *int, includeLower, includeUpper bool) func(interface{}) error {
	checker := func(e interface{}) error {
		i, ok := e.(int)
		if !ok {
			return fmt.Errorf("expected type float32, received %T", e)
		}

		// NOTE(tony): nil means no boundary
		if lower != nil {
			if includeLower && !(*lower <= i) {
				return fmt.Errorf("range check %v <= %v failed", *lower, i)
			}
			if !includeLower && !(*lower < i) {
				return fmt.Errorf("range check %v < %v failed", *lower, i)
			}
		}

		// NOTE(tony): nil means no boundary
		if upper != nil {
			if includeUpper && !(i <= *upper) {
				return fmt.Errorf("range check %v <= %v failed", i, *upper)
			}
			if !includeUpper && !(i < *upper) {
				return fmt.Errorf("range check %v < %v failed", i, *upper)
			}
		}

		return nil
	}

	return checker
}
