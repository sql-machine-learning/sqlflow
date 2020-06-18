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
// TODO(sneaxiy): change the returned value type to be func(float32, string) error
func Float32RangeChecker(lower, upper float32, includeLower, includeUpper bool) func(interface{}, string) error {
	return func(attr interface{}, name string) error {
		if f, ok := attr.(float32); ok {
			e := Float32LowerBoundChecker(lower, includeLower)(f, name)
			if e == nil {
				e = Float32UpperBoundChecker(upper, includeUpper)(f, name)
			}
			return e
		}
		return fmt.Errorf("attribute %s expected type float32, received %T", name, attr)
	}
}

// Float32LowerBoundChecker returns a range checker that only checks the lower bound.
func Float32LowerBoundChecker(lower float32, includeLower bool) func(interface{}, string) error {
	return func(attr interface{}, name string) error {
		if f, ok := attr.(float32); ok {
			if (!includeLower && f > lower) || (includeLower && f >= lower) {
				return nil
			}
			return fmt.Errorf("attribute %s range check %v <%v %v failed", name, lower, equalSign[includeLower], f)
		}
		return fmt.Errorf("attribute %s expected type float32, received %T", name, attr)
	}
}

// Float32UpperBoundChecker returns a range checker that only checks the upper bound.
func Float32UpperBoundChecker(upper float32, includeUpper bool) func(interface{}, string) error {
	return func(attr interface{}, name string) error {
		if f, ok := attr.(float32); ok {
			if (!includeUpper && f < upper) || (includeUpper && f <= upper) {
				return nil
			}
			return fmt.Errorf("attribute %s range check %v >%v %v failed", name, upper, equalSign[includeUpper], f)
		}
		return fmt.Errorf("attribute %s expected type float32, received %T", name, attr)
	}
}

// IntRangeChecker is a helper function to generate range checkers on attributes.
// lower/upper indicates the lower bound and upper bound of the attribute value.
// includeLower/includeUpper indicates the inclusion of the bound.
func IntRangeChecker(lower, upper int, includeLower, includeUpper bool) func(interface{}, string) error {
	return func(attr interface{}, name string) error {
		if f, ok := attr.(int); ok {
			e := IntLowerBoundChecker(lower, includeLower)(f, name)
			if e == nil {
				e = IntUpperBoundChecker(upper, includeUpper)(f, name)
			}
			return e
		}
		return fmt.Errorf("attribute %s expected type int, received %T", name, attr)
	}
}

// IntLowerBoundChecker returns a range checker that only checks the lower bound.
func IntLowerBoundChecker(lower int, includeLower bool) func(interface{}, string) error {
	return func(attr interface{}, name string) error {
		if f, ok := attr.(int); ok {
			if f > lower || includeLower && f == lower {
				return nil
			}
			return fmt.Errorf("attribute %s range check %v <%v %v failed", name, lower, equalSign[includeLower], f)
		}
		return fmt.Errorf("attribute %s expected type int, received %T", name, attr)
	}
}

// IntUpperBoundChecker returns a range checker that only checks the upper bound.
func IntUpperBoundChecker(upper int, includeUpper bool) func(interface{}, string) error {
	return func(attr interface{}, name string) error {
		if f, ok := attr.(int); ok {
			if f < upper || includeUpper && f == upper {
				return nil
			}
			return fmt.Errorf("attribute %s range check %v >%v %v failed", name, upper, equalSign[includeUpper], f)
		}
		return fmt.Errorf("attribute %s expected type int, received %T", name, attr)
	}
}

// IntChoicesChecker verifies the attribute value is in a list of choices.
func IntChoicesChecker(choices ...int) func(interface{}, string) error {
	checker := func(e interface{}, name string) error {
		i, ok := e.(int)
		if !ok {
			return fmt.Errorf("attribute %s expected type int, received %T", name, e)
		}
		for _, possibleValue := range choices {
			if i == possibleValue {
				return nil
			}
		}
		return fmt.Errorf("attribute %s expected value in %v, actual: %v", name, choices, i)
	}
	return checker
}

// StringChoicesChecker verifies the attribute value is in a list of choices.
func StringChoicesChecker(choices ...string) func(interface{}, string) error {
	checker := func(e interface{}, name string) error {
		s, ok := e.(string)
		if !ok {
			return fmt.Errorf("attribute %s expected type string, received %T", name, e)
		}
		for _, possibleValue := range choices {
			if s == possibleValue {
				return nil
			}
		}
		return fmt.Errorf("attribute %s expected value in %v, actual: %v", name, choices, s)
	}
	return checker
}
