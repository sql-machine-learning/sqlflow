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

package ast

import "fmt"

// Attributes is a map from attribute name to the value expression.
type Attributes map[string]*Expr

// Union adds the parameter Attributes into the caller.  It returns error if
// there is any duplicated attribute.
func (as1 Attributes) Union(as2 Attributes) (Attributes, error) {
	for k, v := range as2 {
		if _, ok := as1[k]; ok {
			return nil, fmt.Errorf("attribute %v already exists", k)
		}
		as1[k] = v
	}
	return as1, nil
}
