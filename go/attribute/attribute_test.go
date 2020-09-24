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
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDictionaryNamedTypeChecker(t *testing.T) {
	a := assert.New(t)
	name := "any_attr_name"

	assertFunc := func(d Dictionary, value interface{}, ok bool) {
		err := d.Validate(map[string]interface{}{name: value})
		if ok {
			a.NoError(err)
		} else {
			a.Error(err)
		}
	}

	boolDictWithoutChecker := Dictionary{}.Bool(name, false, "", nil)
	assertFunc(boolDictWithoutChecker, "abc", false)
	assertFunc(boolDictWithoutChecker, false, true)
	assertFunc(boolDictWithoutChecker, true, true)
	boolDictWithChecker := Dictionary{}.Bool(name, nil, "", func(v bool) error {
		if v {
			return fmt.Errorf("attribute %s must be false", name)
		}
		return nil
	})
	assertFunc(boolDictWithChecker, "abc", false)
	assertFunc(boolDictWithChecker, false, true)
	assertFunc(boolDictWithChecker, true, false)

	intDictWithoutChecker := Dictionary{}.Int(name, 0, "", nil)
	assertFunc(intDictWithoutChecker, "abc", false)
	assertFunc(intDictWithoutChecker, 3, true)
	intDictWithChecker := Dictionary{}.Int(name, 0, "", func(v int) error {
		if v == 3 {
			return fmt.Errorf("attribute %s cannot be 3", name)
		}
		return nil
	})
	assertFunc(intDictWithChecker, "abc", false)
	assertFunc(intDictWithChecker, 3, false)
	assertFunc(intDictWithChecker, 0, true)

	floatDictWithoutChecker := Dictionary{}.Float(name, nil, "", nil)
	assertFunc(floatDictWithoutChecker, "abc", false)
	assertFunc(floatDictWithoutChecker, float32(-1.5), true)
	floatDictWithChecker := Dictionary{}.Float(name, float32(0), "", func(v float32) error {
		if v <= float32(-1.0) {
			return fmt.Errorf("attribute %s must larger than -1.0", name)
		}
		return nil
	})
	assertFunc(floatDictWithChecker, "abc", false)
	assertFunc(floatDictWithChecker, float32(-2.0), false)
	assertFunc(floatDictWithChecker, float32(7.5), true)

	stringDictWithoutChecker := Dictionary{}.String(name, "", "", nil)
	assertFunc(stringDictWithoutChecker, 1, false)
	assertFunc(stringDictWithoutChecker, "abc", true)
	stringDictWithChecker := Dictionary{}.String(name, nil, "", func(v string) error {
		if !strings.HasPrefix(v, "valid") {
			return fmt.Errorf("attribute %s must have prefix valid", name)
		}
		return nil
	})
	assertFunc(stringDictWithChecker, 1, false)
	assertFunc(stringDictWithChecker, "invalidString", false)
	assertFunc(stringDictWithChecker, "validString", true)

	intListDictWithoutChecker := Dictionary{}.IntList(name, []int{}, "", nil)
	assertFunc(intListDictWithoutChecker, "abc", false)
	assertFunc(intListDictWithoutChecker, []int{1}, true)
	intListDictWithChecker := Dictionary{}.IntList(name, []int{}, "", func(v []int) error {
		if len(v) > 2 {
			return fmt.Errorf("length of attribute %s must be less than or equal to 2", name)
		}
		return nil
	})
	assertFunc(intListDictWithChecker, "abc", false)
	assertFunc(intListDictWithChecker, []int{1, 2, 3}, false)
	assertFunc(intListDictWithChecker, []int{1, 2}, true)

	unknownTypeDictWithoutChecker := Dictionary{}.Unknown(name, nil, "", nil)
	assertFunc(unknownTypeDictWithoutChecker, 1, true)
	assertFunc(unknownTypeDictWithoutChecker, float32(-0.5), true)
	assertFunc(unknownTypeDictWithoutChecker, "abc", true)

	unknownTypeDictWithChecker := Dictionary{}.Unknown(name, 1, "", func(v interface{}) error {
		if _, ok := v.(int); ok {
			return nil
		}
		if _, ok := v.(string); ok {
			return nil
		}
		return fmt.Errorf("attribute %s must be of type int or string", name)
	})
	assertFunc(unknownTypeDictWithChecker, 1, true)
	assertFunc(unknownTypeDictWithChecker, "abc", true)
	assertFunc(unknownTypeDictWithChecker, float32(1.5), false)
}

func TestDictionaryValidate(t *testing.T) {
	a := assert.New(t)

	checker := func(i int) error {
		if i < 0 {
			return fmt.Errorf("some error")
		}
		return nil
	}
	tb := Dictionary{}.Int("a", 1, "attribute a", checker).Float("b", float32(1), "attribute b", nil)
	a.NoError(tb.Validate(map[string]interface{}{"a": 1}))
	a.EqualError(tb.Validate(map[string]interface{}{"a": -1}), "attribute a error: some error")
	a.EqualError(tb.Validate(map[string]interface{}{"_a": -1}), fmt.Sprintf(errUnsupportedAttribute, "_a"))
	a.EqualError(tb.Validate(map[string]interface{}{"a": 1.0}), "attribute a must be of type int, but got float64")
	a.NoError(tb.Validate(map[string]interface{}{"b": float32(1.0)}))
	a.NoError(tb.Validate(map[string]interface{}{"b": 1}))
}

func TestParamsDocs(t *testing.T) {
	a := assert.New(t)

	a.Equal(11, len(PremadeModelParamsDocs))
	ExtractSQLFlowModelsSymbolOnce()
	a.Equal(23, len(PremadeModelParamsDocs))
	a.Equal(len(PremadeModelParamsDocs["DNNClassifier"]), 12)
	a.NotContains(PremadeModelParamsDocs["DNNClassifier"], "feature_columns")
	a.Contains(PremadeModelParamsDocs["DNNClassifier"], "optimizer")
	a.Contains(PremadeModelParamsDocs["DNNClassifier"], "hidden_units")
	a.Contains(PremadeModelParamsDocs["DNNClassifier"], "n_classes")

	a.True(reflect.DeepEqual(PremadeModelParamsDocs["xgboost.gbtree"], PremadeModelParamsDocs["xgboost.dart"]))
	a.True(reflect.DeepEqual(PremadeModelParamsDocs["xgboost.dart"], PremadeModelParamsDocs["xgboost.gblinear"]))
	a.Equal(24, len(PremadeModelParamsDocs["xgboost.gbtree"]))
	a.NotContains(PremadeModelParamsDocs["DNNClassifier"], "booster")

	a.Equal(8, len(OptimizerParamsDocs))
}

func TestNewAndUpdateDictionary(t *testing.T) {
	a := assert.New(t)

	commonAttrs := Dictionary{}.Int("a", 1, "attribute a", nil)
	specificAttrs := NewDictionaryFromModelDefinition("DNNClassifier", "model.")
	a.Equal(len(specificAttrs), 12)
	a.Equal(len(specificAttrs.Update(specificAttrs)), 12)
	a.Equal(len(specificAttrs.Update(commonAttrs)), 13)
	a.Equal(len(specificAttrs), 13)
	a.True(reflect.DeepEqual(specificAttrs["a"], commonAttrs["a"]))
	a.Contains(specificAttrs, "model.optimizer")
	a.Contains(specificAttrs, "model.hidden_units")
	a.Contains(specificAttrs, "model.n_classes")
	a.NotContains(specificAttrs, "model.feature_columns")
}

func TestDictionary_GenerateTableInHTML(t *testing.T) {
	a := assert.New(t)
	tb := Dictionary{}.
		Int("a", 1, `this is a
multiple line
doc string.`, nil).
		String("世界", "", `42`, nil)

	expected := `<table>
<tr>
	<td>Name</td>
	<td>Type</td>
	<td>Description</td>
</tr>
<tr>
	<td>a</td>
	<td>int</td>
	<td>this is a<br>multiple line<br>doc string.</td>
</tr>
<tr>
	<td>世界</td>
	<td>string</td>
	<td>42</td>
</tr>
</table>`
	actual := tb.GenerateTableInHTML()
	a.Equal(expected, actual)
}
