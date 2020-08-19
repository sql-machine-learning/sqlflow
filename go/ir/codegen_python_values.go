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

package ir

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DTypeToString returns string value of dtype
func DTypeToString(dt int) string {
	switch dt {
	case Float:
		return "float32"
	case Int:
		return "int64"
	case String:
		return "string"
	default:
		return ""
	}
}

// AttrToPythonValue format the WITH attributes to corresponding Python code.
func AttrToPythonValue(attr interface{}) string {
	switch a := attr.(type) {
	case bool:
		return strings.Title(fmt.Sprintf("%v", a))
	case int:
		return fmt.Sprintf("%d", a)
	case int64:
		return fmt.Sprintf("%d", a)
	case float32:
		return fmt.Sprintf("%f", a)
	case float64: // FIXME(typhoonzero): may never use
		return fmt.Sprintf("%f", attr.(float64))
	case []int:
		if a == nil {
			return "None"
		}
		intArrayAttrStr, _ := MarshalToJSONString(a)
		return intArrayAttrStr
	case []string:
		if a == nil {
			return "None"
		}
		if len(a) == 0 {
			return "[]"
		}
		stringListStr, _ := MarshalToJSONString(a)
		return stringListStr
	case []interface{}:
		if a == nil {
			return "None"
		}
		if len(a) > 0 {
			if _, ok := a[0].(int); ok {
				intlist := []int{}
				for _, v := range a {
					intlist = append(intlist, v.(int))
				}
				intlistStr, _ := MarshalToJSONString(intlist)
				return intlistStr
			}
		}
		// TODO(typhoonzero): support []float etc.
		return "[]"
	case string:
		return fmt.Sprintf(`"%s"`, a)
	default:
		return ""
	}
}

// MarshalToJSONString converts any data to JSON string.
func MarshalToJSONString(in interface{}) (string, error) {
	bytes, err := json.Marshal(in)
	return string(bytes), err
}
