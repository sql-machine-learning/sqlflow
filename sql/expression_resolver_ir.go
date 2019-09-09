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

package sql

import (
	"fmt"
	"strconv"
)

func resolveAttributeForIR(attrs *attrs) (map[string]interface{}, error) {
	ret := make(map[string]interface{})
	for k, v := range *attrs {
		r, err := resolveAttributeExpressionForIR(v)
		if err != nil {
			return nil, err
		}
		ret[k] = r
		fmt.Printf("%v %T %v %T\n", k, k, r, r)
	}
	return ret, nil
}

func resolveAttributeExpressionForIR(e *expr) (interface{}, error) {
	if len(e.val) == 0 {
		el := e.sexp
		if el[0].typ != '[' {
			return nil, fmt.Errorf("attribute expression only supports int, float, string, and list. received %s", e.String())
		}
		var list []int
		for idx, expr := range el {
			if idx > 0 {
				if expr.sexp != nil {
					return nil, fmt.Errorf("attribute expression list element only supports integer. received %s", e.String())
				}
				intVal, err := strconv.Atoi(expr.val)
				if err != nil {
					return nil, err
				}
				list = append(list, intVal)
			}
		}
		return list, nil
	}

	switch e.typ {
	case NUMBER:
		if v, err := strconv.ParseInt(e.val, 10, 64); err == nil {
			return v, nil
		}
		if v, err := strconv.ParseFloat(e.val, 64); err == nil {
			return v, nil
		}
		return nil, fmt.Errorf("convert attribute %s to float64/int64 failed", e.val)
	case STRING:
		return e.val[1 : len(e.val)-1], nil
	default:
		return nil, fmt.Errorf("invalid attribute %s", e.val)
	}
}
