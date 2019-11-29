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
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

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
	// Unknown type indicates that the attribute type is dynamically determined.
	Unknown
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
		var desc *Description
		desc, ok := d[k]
		if !ok {
			// Support attribute definition like "model.*" to match attributes start with "model"
			keyParts := strings.Split(k, ".")
			if len(keyParts) == 2 {
				wildCard := fmt.Sprintf("%s.*", keyParts[0])
				descWild, okWildCard := d[wildCard]
				if okWildCard {
					desc = descWild
				} else {
					return fmt.Errorf(errUnsupportedAttribute, k)
				}
			} else {
				return fmt.Errorf(errUnsupportedAttribute, k)
			}

		}
		// unknown type of attribute do not need to run validate
		if desc.Type == Unknown {
			if desc.Checker != nil {
				if err := desc.Checker(v); err != nil {
					return err
				}
			}
			continue
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

// GenerateTableInHTML generates the attribute dictionary table in HTML format
func (d Dictionary) GenerateTableInHTML() string {
	l := make([]string, 0)
	l = append(l, `<table>`)
	l = append(l, `<tr>
	<td>Name</td>
	<td>Type</td>
	<td>Description</td>
</tr>`)

	// the rows are sorted according key names
	keys := make([]string, 0)
	for k := range d {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		desc := d[k]
		t := `<tr>
	<td>%s</td>
	<td>%s</td>
	<td>%s</td>
</tr>`
		// NOTE(tony): if the doc string has multiple lines, need to replace \n with <br>
		s := fmt.Sprintf(t, k, desc.Type.String(), strings.Replace(desc.Doc, "\n", `<br>`, -1))
		l = append(l, s)
	}

	l = append(l, `</table>`)
	return strings.Join(l, "\n")
}

// Update updates `d` by copying from `other` key by key
func (d Dictionary) Update(other Dictionary) Dictionary {
	for k, v := range other {
		d[k] = v
	}
	return d
}

// NewDictionary create a new Dictionary according to `estimator`
func NewDictionary(estimator, prefix string) Dictionary {
	var d = Dictionary{}
	for param, doc := range PremadeModelParamsDocs[estimator] {
		d[prefix+param] = &Description{Unknown, doc, nil}
	}
	return d
}

// PremadeModelParamsDocs stores parameters and documents of all known models
var PremadeModelParamsDocs map[string]map[string]string

func init() {
	if err := json.Unmarshal([]byte(ModelParameterJSON), &PremadeModelParamsDocs); err != nil {
		panic(err) // assertion
	}
	// The following parameters of canned estimators are already supported in the COLUMN clause.
	for _, v := range PremadeModelParamsDocs {
		delete(v, "feature_columns")
		delete(v, "dnn_feature_columns")
		delete(v, "linear_feature_columns")
	}
}
