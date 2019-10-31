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

var equalSign = map[bool]string{true: "=", false: ""}

// Float32RangeChecker is a helper functions to generate range checkers on attributes.
// lower/upper indicates the lower bound and upper bound of the attribute value.
// includeLower/includeUpper indicates the inclusion of the bound.
func Float32RangeChecker(lower, upper float32, includeLower, includeUpper bool) func(interface{}) error {
	return func(attr interface{}) error {
		if f, ok := attr.(float32); ok {
			e := Float32LowerBoundChecker(lower, includeLower)(f)
			if e == nil {
				e = Float32UpperBoundChecker(upper, includeUpper)(f)
			}
			return e
		}
		return fmt.Errorf("expected type float32, received %T", attr)
	}
}

// Float32LowerBoundChecker returns a range checker that only checks the lower bound
func Float32LowerBoundChecker(lower float32, includeLower bool) func(interface{}) error {
	return func(attr interface{}) error {
		if f, ok := attr.(float32); ok {
			if f > lower || includeLower && f == lower {
				return nil
			}
			return fmt.Errorf("range check %v <%v %v failed", lower, equalSign[includeLower], f)
		}
		return fmt.Errorf("expected type float32, received %T", attr)
	}
}

// Float32UpperBoundChecker returns a range checker that only checks the upper bound
func Float32UpperBoundChecker(upper float32, includeUpper bool) func(interface{}) error {
	return func(attr interface{}) error {
		if f, ok := attr.(float32); ok {
			if f < upper || includeUpper && f == upper {
				return nil
			}
			return fmt.Errorf("range check %v >%v %v failed", upper, equalSign[includeUpper], f)
		}
		return fmt.Errorf("expected type float32, received %T", attr)
	}
}

// IntRangeChecker is a helper functions to generate range checkers on attributes.
// lower/upper indicates the lower bound and upper bound of the attribute value.
// includeLower/includeUpper indicates the inclusion of the bound.
func IntRangeChecker(lower, upper int, includeLower, includeUpper bool) func(interface{}) error {
	return func(attr interface{}) error {
		if f, ok := attr.(int); ok {
			e := IntLowerBoundChecker(lower, includeLower)(f)
			if e == nil {
				e = IntUpperBoundChecker(upper, includeUpper)(f)
			}
			return e
		}
		return fmt.Errorf("expected type int, received %T", attr)
	}
}

// IntLowerBoundChecker returns a range checker that only checks the lower bound
func IntLowerBoundChecker(lower int, includeLower bool) func(interface{}) error {
	return func(attr interface{}) error {
		if f, ok := attr.(int); ok {
			if f > lower || includeLower && f == lower {
				return nil
			}
			return fmt.Errorf("range check %v <%v %v failed", lower, equalSign[includeLower], f)
		}
		return fmt.Errorf("expected type int, received %T", attr)
	}
}

// IntUpperBoundChecker returns a range checker that only checks the upper bound
func IntUpperBoundChecker(upper int, includeUpper bool) func(interface{}) error {
	return func(attr interface{}) error {
		if f, ok := attr.(int); ok {
			if f < upper || includeUpper && f == upper {
				return nil
			}
			return fmt.Errorf("range check %v >%v %v failed", upper, equalSign[includeUpper], f)
		}
		return fmt.Errorf("expected type int, received %T", attr)
	}
}

// EmptyChecker returns a checker function that do **not** check the input.
func EmptyChecker() func(interface{}) error {
	checker := func(e interface{}) error {
		return nil
	}
	return checker
}
