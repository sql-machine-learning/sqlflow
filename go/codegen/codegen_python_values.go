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

package codegen

import (
	"fmt"
	"strings"

	"sqlflow.org/sqlflow/go/ir"
)

// DTypeToString returns string value of dtype
func DTypeToString(dt int) string {
	switch dt {
	case ir.Float:
		return "float32"
	case ir.Int:
		return "int64"
	case ir.String:
		return "string"
	default:
		return ""
	}
}

// AttrToPythonValue format the WITH attributes to corresponding Python code.
func AttrToPythonValue(attr interface{}) string {
	switch attr.(type) {
	case bool:
		return strings.Title(fmt.Sprintf("%v", attr.(bool)))
	case int:
		return fmt.Sprintf("%d", attr.(int))
	case int64:
		return fmt.Sprintf("%d", attr.(int64))
	case float32:
		return fmt.Sprintf("%f", attr.(float32))
	case float64: // FIXME(typhoonzero): may never use
		return fmt.Sprintf("%f", attr.(float64))
	case []int:
		intArrayAttrStr, _ := MarshalToJSONString(attr.([]int))
		return intArrayAttrStr
	case []string:
		l := attr.([]string)
		if len(l) == 0 {
			return "[]"
		}
		stringListStr, _ := MarshalToJSONString(l)
		return stringListStr
	case []interface{}:
		tmplist := attr.([]interface{})
		if len(tmplist) > 0 {
			if _, ok := tmplist[0].(int); ok {
				intlist := []int{}
				for _, v := range tmplist {
					intlist = append(intlist, v.(int))
				}
				intlistStr, _ := MarshalToJSONString(intlist)
				return intlistStr
			}
		}
		// TODO(typhoonzero): support []float etc.
		return "[]"
	case string:
		return fmt.Sprintf("\"%s\"", attr.(string))
	default:
		return ""
	}
}
