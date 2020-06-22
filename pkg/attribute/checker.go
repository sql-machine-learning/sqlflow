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
	"fmt"
)

var equalSign = map[bool]string{true: "=", false: ""}

// Float32RangeChecker is a helper function to generate range checkers on attributes.
// lower/upper indicates the lower bound and upper bound of the attribute value.
// includeLower/includeUpper indicates the inclusion of the bound.
func Float32RangeChecker(lower, upper float32, includeLower, includeUpper bool) func(float32) error {
	return func(f float32) error {
		e := Float32LowerBoundChecker(lower, includeLower)(f)
		if e == nil {
			e = Float32UpperBoundChecker(upper, includeUpper)(f)
		}
		return e
	}
}

// Float32LowerBoundChecker returns a range checker that only checks the lower bound.
func Float32LowerBoundChecker(lower float32, includeLower bool) func(float32) error {
	return func(f float32) error {
		if (!includeLower && f > lower) || (includeLower && f >= lower) {
			return nil
		}
		return fmt.Errorf("range check %v <%v %v failed", lower, equalSign[includeLower], f)
	}
}

// Float32UpperBoundChecker returns a range checker that only checks the upper bound.
func Float32UpperBoundChecker(upper float32, includeUpper bool) func(float32) error {
	return func(f float32) error {
		if (!includeUpper && f < upper) || (includeUpper && f <= upper) {
			return nil
		}
		return fmt.Errorf("range check %v >%v %v failed", upper, equalSign[includeUpper], f)
	}
}

// IntRangeChecker is a helper function to generate range checkers on attributes.
// lower/upper indicates the lower bound and upper bound of the attribute value.
// includeLower/includeUpper indicates the inclusion of the bound.
func IntRangeChecker(lower, upper int, includeLower, includeUpper bool) func(int) error {
	return func(i int) error {
		e := IntLowerBoundChecker(lower, includeLower)(i)
		if e == nil {
			e = IntUpperBoundChecker(upper, includeUpper)(i)
		}
		return e
	}
}

// IntLowerBoundChecker returns a range checker that only checks the lower bound.
func IntLowerBoundChecker(lower int, includeLower bool) func(int) error {
	return func(i int) error {
		if i > lower || includeLower && i == lower {
			return nil
		}
		return fmt.Errorf("range check %v <%v %v failed", lower, equalSign[includeLower], i)
	}
}

// IntUpperBoundChecker returns a range checker that only checks the upper bound.
func IntUpperBoundChecker(upper int, includeUpper bool) func(int) error {
	return func(i int) error {
		if i < upper || includeUpper && i == upper {
			return nil
		}
		return fmt.Errorf("range check %v >%v %v failed", upper, equalSign[includeUpper], i)
	}
}

// IntChoicesChecker verifies the attribute value is in a list of choices.
func IntChoicesChecker(choices ...int) func(int) error {
	return func(i int) error {
		for _, possibleValue := range choices {
			if i == possibleValue {
				return nil
			}
		}
		return fmt.Errorf("expected value in %v, actual: %v", choices, i)
	}
}

// StringChoicesChecker verifies the attribute value is in a list of choices.
func StringChoicesChecker(choices ...string) func(string) error {
	return func(s string) error {
		for _, possibleValue := range choices {
			if s == possibleValue {
				return nil
			}
		}
		return fmt.Errorf("expected value in %v, actual: %v", choices, s)
	}
}
