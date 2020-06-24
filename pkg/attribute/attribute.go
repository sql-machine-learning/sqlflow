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
	"regexp"
	"sort"
	"strings"
	"sync"
)

const (
	errUnsupportedAttribute = "unsupported attribute %v"
)

var (
	// boolType indicates that the corresponding attribute is a boolean
	boolType = reflect.TypeOf(true)
	// intType indicates that the corresponding attribute is an integer
	intType = reflect.TypeOf(0)
	// floatType indicates that the corresponding attribute is a float32
	floatType = reflect.TypeOf(float32(0.))
	// stringType indicates the corresponding attribute is a string
	stringType = reflect.TypeOf("")
	// intListType indicates the corresponding attribute is a list of integers
	intListType = reflect.TypeOf([]int{})
	// unknownType indicates that the attribute type is dynamically determined.
	unknownType = reflect.Type(nil)
)

// Dictionary contains the description of all attributes
type Dictionary map[string]*description

// description describes a requirement for a particular attribute
type description struct {
	typ          reflect.Type
	defaultValue interface{}
	doc          string
	checker      func(value interface{}, name string) error
}

// Int declares an attribute of int-typed in Dictionary d.
func (d Dictionary) Int(name string, value interface{}, doc string, checker func(int) error) Dictionary {
	interfaceChecker := func(value interface{}, name string) error {
		if intValue, ok := value.(int); ok {
			if checker != nil {
				if err := checker(intValue); err != nil {
					return fmt.Errorf("attribute %s error: %s", name, err)
				}
			}
			return nil
		}
		return fmt.Errorf("attribute %s must be of type int, but got %T", name, value)
	}

	if value != nil {
		err := interfaceChecker(value, name)
		if err != nil {
			log.Panicf("default value of attribute %s is invalid, error is: %s", name, err)
		}
	}

	d[name] = &description{
		typ:          intType,
		defaultValue: value,
		doc:          doc,
		checker:      interfaceChecker,
	}
	return d
}

// Float declares an attribute of float32-typed in Dictionary d.
func (d Dictionary) Float(name string, value interface{}, doc string, checker func(float32) error) Dictionary {
	interfaceChecker := func(value interface{}, name string) error {
		var fValue float32
		if floatValue, ok := value.(float32); ok {
			fValue = floatValue
		} else if intValue, ok := value.(int); ok { // implicit type conversion from int to float
			fValue = float32(intValue)
		} else {
			return fmt.Errorf("attribute %s must be of type float, but got %T", name, value)
		}

		if checker != nil {
			if err := checker(fValue); err != nil {
				return fmt.Errorf("attribute %s error: %s", name, err)
			}
		}
		return nil
	}

	if value != nil {
		err := interfaceChecker(value, name)
		if err != nil {
			log.Panicf("default value of attribute %s is invalid, error is: %s", name, err)
		}
	}

	var fInterfaceValue interface{}
	if value == nil {
		fInterfaceValue = nil
	} else if floatValue, ok := value.(float32); ok {
		fInterfaceValue = floatValue
	} else if intValue, ok := value.(int); ok { // implicit type conversion from int to float
		fInterfaceValue = float32(intValue)
	}

	d[name] = &description{
		typ:          floatType,
		defaultValue: fInterfaceValue,
		doc:          doc,
		checker:      interfaceChecker,
	}
	return d
}

// Bool declares an attribute of bool-typed in Dictionary d.
func (d Dictionary) Bool(name string, value interface{}, doc string, checker func(bool) error) Dictionary {
	interfaceChecker := func(value interface{}, name string) error {
		if boolValue, ok := value.(bool); ok {
			if checker != nil {
				if err := checker(boolValue); err != nil {
					return fmt.Errorf("attribute %s error: %s", name, err)
				}
			}
			return nil
		}
		return fmt.Errorf("attribute %s must be of type bool, but got %T", name, value)
	}

	if value != nil {
		err := interfaceChecker(value, name)
		if err != nil {
			log.Panicf("default value of attribute %s is invalid, error is: %s", name, err)
		}
	}

	d[name] = &description{
		typ:          boolType,
		defaultValue: value,
		doc:          doc,
		checker:      interfaceChecker,
	}
	return d
}

// String declares an attribute of string-typed in Dictionary d.
func (d Dictionary) String(name string, value interface{}, doc string, checker func(string) error) Dictionary {
	interfaceChecker := func(value interface{}, name string) error {
		if stringValue, ok := value.(string); ok {
			if checker != nil {
				if err := checker(stringValue); err != nil {
					return fmt.Errorf("attribute %s error: %s", name, err)
				}
			}
			return nil
		}
		return fmt.Errorf("attribute %s must be of type string, but got %T", name, value)
	}

	if value != nil {
		err := interfaceChecker(value, name)
		if err != nil {
			log.Panicf("default value of attribute %s is invalid, error is: %s", name, err)
		}
	}

	d[name] = &description{
		typ:          stringType,
		defaultValue: value,
		doc:          doc,
		checker:      interfaceChecker,
	}
	return d
}

// IntList declares an attribute of []int-typed in Dictionary d.
func (d Dictionary) IntList(name string, value interface{}, doc string, checker func([]int) error) Dictionary {
	interfaceChecker := func(value interface{}, name string) error {
		if intListValue, ok := value.([]int); ok {
			if checker != nil {
				if err := checker(intListValue); err != nil {
					return fmt.Errorf("attribute %s error: %s", name, err)
				}
			}
			return nil
		}
		return fmt.Errorf("attribute %s must be of type []int, but got %T", name, value)
	}

	if value != nil {
		err := interfaceChecker(value, name)
		if err != nil {
			log.Panicf("default value of attribute %s is invalid, error is: %s", name, err)
		}
	}

	d[name] = &description{
		typ:          intListType,
		defaultValue: value,
		doc:          doc,
		checker:      interfaceChecker,
	}
	return d
}

// Unknown declares an attribute of dynamically determined type
func (d Dictionary) Unknown(name string, value interface{}, doc string, checker func(interface{}) error) Dictionary {
	var interfaceChecker func(value interface{}, name string) error

	if checker != nil {
		interfaceChecker = func(value interface{}, name string) error {
			if err := checker(value); err != nil {
				return fmt.Errorf("attribute %s error: %s", name, err)
			}
			return nil
		}
	}

	if value != nil && interfaceChecker != nil {
		err := interfaceChecker(value, name)
		if err != nil {
			log.Panicf("default value of attribute %s is invalid, error is: %s", name, err)
		}
	}

	d[name] = &description{
		typ:          unknownType,
		defaultValue: value,
		doc:          doc,
		checker:      interfaceChecker,
	}
	return d
}

// ExportDefaults exports default values defined in Dictionary to attrs.
func (d Dictionary) ExportDefaults(attrs map[string]interface{}) {
	for k, v := range d {
		// Do not fill default value for unknown type, and with nil default values.
		if v.typ == unknownType {
			continue
		}
		if v.defaultValue == nil {
			continue
		}
		_, ok := attrs[k]
		if !ok {
			attrs[k] = v.defaultValue
		}
	}
}

// Validate validates the attribute based on dictionary. The validation includes
//   1. Type checking
//   2. Customer checker
func (d Dictionary) Validate(attrs map[string]interface{}) error {
	for k, v := range attrs {
		var desc *description
		desc, ok := d[k]
		if !ok {
			// Support attribute definition like "model.*" to match
			// attributes start with "model"
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

		if v != nil && desc.checker != nil {
			if err := desc.checker(v, k); err != nil {
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

		var typ interface{} = desc.typ
		if typ == nil {
			typ = "unknown"
		}

		// NOTE(tony): if the doc string has multiple lines, need to replace \n with <br>
		s := fmt.Sprintf(t, k, typ, strings.Replace(desc.doc, "\n", `<br>`, -1))
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
	isXGBoostModel := strings.HasPrefix(estimator, "xgboost")
	re := regexp.MustCompile("[^a-z]")

	var d = Dictionary{}
	for param, doc := range PremadeModelParamsDocs[estimator] {
		desc := &description{unknownType, nil, doc, nil}
		d[prefix+param] = desc

		if !isXGBoostModel {
			continue
		}

		// Fill typ field according to the model parameter doc
		// The doc would be like: "int Maximum tree depth for base learners"
		pieces := strings.SplitN(strings.TrimSpace(desc.doc), " ", 2)
		if len(pieces) != 2 {
			continue
		}

		maybeType := re.ReplaceAllString(pieces[0], "")
		switch strings.ToLower(maybeType) {
		case "float":
			desc.typ = floatType
			desc.doc = pieces[1]
		case "int":
			desc.typ = intType
			desc.doc = pieces[1]
		case "string":
			desc.typ = stringType
			desc.doc = pieces[1]
		case "boolean":
			desc.typ = boolType
			desc.doc = pieces[1]
		}
	}
	return d
}

// ExtractSymbol extracts parameter documents of Python modules from doc strings
func ExtractSymbol(module ...string) {
	cmd := exec.Command("python", "-uc", fmt.Sprintf("__import__('symbol_extractor').print_param_doc('%s')", strings.Join(module, "', '")))
	output, e := cmd.CombinedOutput()
	if e != nil {
		log.Println("ExtractSymbol failed: ", e, string(output))
	}
	// json.Unmarshal extends the map rather than reallocate a new one, see golang.org/pkg/encoding/json/#Unmarshal
	if e := json.Unmarshal(output, &PremadeModelParamsDocs); e != nil {
		log.Println("ExtractSymbol failed:", e, string(output))
	}
}

var extractSQLFlowModelsSymbolOnce sync.Once

// ExtractSQLFlowModelsSymbolOnce extracts parameter documents from python doc strings using sync.Once
func ExtractSQLFlowModelsSymbolOnce() {
	extractSQLFlowModelsSymbolOnce.Do(func() { ExtractSymbol("sqlflow_models") })
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
	removeUnnecessaryParams()
}
