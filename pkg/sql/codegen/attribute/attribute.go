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

import "fmt"

// Type indicates the attribute type of an attribute in the WITH clause
type Type int

const (
	errUnsupportedAttribute = "unsupported attribute %v"
	errUnexpectedType       = `unexpected type on attribute %v. expect %s, received %T`
)

const (
	// Int indicates the corresponding attribute is an integer
	Int Type = iota
	// Float indicates the corresponding attribute is a float
	Float
	// String indicates the corresponding attribute is a string
	String
	// IntList indicates the corresponding attribute is a list of integers
	IntList
)

// Dictionary contains the description of all attributes
type Dictionary map[string]*Description

// Description describes a requirement for a particular attribute
type Description struct {
	Type    Type
	Doc     string
	Checker func(i interface{}) error
}

func (t Type) String() string {
	switch t {
	case Int:
		return "Int"
	case Float:
		return "Float"
	case String:
		return "String"
	case IntList:
		return "IntList"
	default:
		return "Unknown"
	}
}

// Validate validates the attribute based on dictionary. The validation includes
//   1. Type checking
//   2. Customer checker
func (d Dictionary) Validate(attrs map[string]interface{}) error {
	for k, v := range attrs {
		desc, ok := d[k]
		if !ok {
			return fmt.Errorf(errUnsupportedAttribute, k)
		}
		switch v.(type) {
		case int, int32, int64:
			if desc.Type != Int {
				return fmt.Errorf(errUnexpectedType, k, desc.Type.String(), v)
			}
		case float32, float64:
			if desc.Type != Float {
				return fmt.Errorf(errUnexpectedType, k, desc.Type.String(), v)
			}
		case string:
			if desc.Type != String {
				return fmt.Errorf(errUnexpectedType, k, desc.Type.String(), v)
			}
		case []int32, []int64:
			if desc.Type != IntList {
				return fmt.Errorf(errUnexpectedType, k, desc.Type.String(), v)
			}
		default:
			return fmt.Errorf(errUnexpectedType, k, "one of Int/Float/String/IntList", v)
		}
		if desc.Checker != nil {
			if err := desc.Checker(v); err != nil {
				return err
			}
		}
	}
	return nil
}

// TODO(tony): Add doc generation functionality. For example
// func (d *Dictionary) GenerateTableInMarkdown() string
