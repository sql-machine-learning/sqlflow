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
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"reflect"
	"sort"
	"strings"
)

const (
	errUnsupportedAttribute = "unsupported attribute %v"
	errUnexpectedType       = `unexpected type on attribute %v. expect %s, received %[3]v(%[3]T)`
)

type unknown struct{}

var (
	// Bool indicates that the corresponding attribute is a boolean
	Bool = reflect.TypeOf(true)
	// Int indicates that the corresponding attribute is an integer
	Int = reflect.TypeOf(0)
	// Float indicates that the corresponding attribute is a float32
	Float = reflect.TypeOf(float32(0.))
	// String indicates the corresponding attribute is a string
	String = reflect.TypeOf("")
	// IntList indicates the corresponding attribute is a list of integers
	IntList = reflect.TypeOf([]int{})
	// Unknown type indicates that the attribute type is dynamically determined.
	Unknown = reflect.TypeOf(unknown{})
)

// Dictionary contains the description of all attributes
type Dictionary map[string]*Description

// Description describes a requirement for a particular attribute
type Description struct {
	Type    reflect.Type
	Default interface{}
	Doc     string
	Checker func(i interface{}) error
}

// FillDefaults fills default values defined in Dictionary to attrs.
func (d Dictionary) FillDefaults(attrs map[string]interface{}) {
	for k, v := range d {
		// Do not fill default value for unknown type, and with nil default values.
		if v.Type == Unknown {
			continue
		}
		if v.Default == nil {
			continue
		}
		_, ok := attrs[k]
		if !ok {
			attrs[k] = v.Default
		}
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

		if desc.Type != Unknown && desc.Type != reflect.TypeOf(v) {
			// Allow implicit converstion from int to float to ease typing
			if !(desc.Type == Float && reflect.TypeOf(v) == Int) {
				return fmt.Errorf(errUnexpectedType, k, desc.Type, v)
			}
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
	l := []string{`<table>`,
		`<tr>
	<td>Name</td>
	<td>Type</td>
	<td>Description</td>
</tr>`}
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
		s := fmt.Sprintf(t, k, desc.Type, strings.Replace(desc.Doc, "\n", `<br>`, -1))
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

// NewDictionaryFromModelDefinition create a new Dictionary according to pre-made estimators or XGBoost model types.
func NewDictionaryFromModelDefinition(estimator, prefix string) Dictionary {
	var d = Dictionary{}
	for param, doc := range PremadeModelParamsDocs[estimator] {
		d[prefix+param] = &Description{Unknown, nil, doc, nil}
	}
	return d
}

// PremadeModelParamsDocs stores parameters and documents of all known models
var PremadeModelParamsDocs map[string]map[string]string

// ExtractDocString extracts parameter documents from python doc strings
func ExtractDocString(module ...string) {
	cmd := exec.Command("python", "-uc", fmt.Sprintf("__import__('extract_docstring').print_param_doc('%s')", strings.Join(module, "', '")))
	output, e := cmd.CombinedOutput()
	if e != nil {
		log.Println("ExtractDocString failed: ", e, string(output))
	}
	// json.Unmarshal extends the map rather than reallocate a new one, see golang.org/pkg/encoding/json/#Unmarshal
	if e := json.Unmarshal(output, &PremadeModelParamsDocs); e != nil {
		log.Println("ExtractDocString failed:", e, string(output))
	}
}

func removeUnnecessaryParams() {
	// The following parameters of canned estimators are already supported in the COLUMN clause.
	for _, v := range PremadeModelParamsDocs {
		delete(v, "feature_columns")
		delete(v, "dnn_feature_columns")
		delete(v, "linear_feature_columns")
	}
}

func init() {
	if err := json.Unmarshal([]byte(ModelParameterJSON), &PremadeModelParamsDocs); err != nil {
		panic(err) // assertion
	}
	ExtractDocString("sqlflow_models")
	removeUnnecessaryParams()
}
