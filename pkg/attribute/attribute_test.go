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

func TestDictionaryValidate(t *testing.T) {
	a := assert.New(t)

	checker := func(i interface{}, name string) error {
		ii, ok := i.(int)
		if !ok {
			return fmt.Errorf("attribute %s %T %v should of type integer", name, i, i)
		}
		if ii < 0 {
			return fmt.Errorf("some error")
		}
		return nil
	}
	tb := Dictionary{"a": {Int, 1, "attribute a", checker}, "b": {Float, 1, "attribute b", nil}}
	a.NoError(tb.Validate(map[string]interface{}{"a": 1}))
	a.EqualError(tb.Validate(map[string]interface{}{"a": -1}), "some error")
	a.EqualError(tb.Validate(map[string]interface{}{"_a": -1}), fmt.Sprintf(errUnsupportedAttribute, "_a"))
	a.EqualError(tb.Validate(map[string]interface{}{"a": 1.0}), fmt.Sprintf(errUnexpectedType, "a", "int", 1.))
	a.NoError(tb.Validate(map[string]interface{}{"b": float32(1.0)}))
	a.NoError(tb.Validate(map[string]interface{}{"b": 1}))
}

func TestDictionaryChecker(t *testing.T) {
	a := assert.New(t)
	var tb Dictionary
	var err error

	boolChecker := func(b bool, name string) error {
		if !b {
			return fmt.Errorf("attribute %s must be true", name)
		}
		return nil
	}
	tb = Dictionary{}.Bool("a", true, "attr a", boolChecker)
	err = tb.Validate(map[string]interface{}{"a": false})
	a.Error(err)
	a.True(strings.HasPrefix(err.Error(), "attribute a"))
	a.NoError(tb.Validate(map[string]interface{}{"a": true}))

	intChecker := func(i int, name string) error {
		if i == 3 {
			return fmt.Errorf("attribute %s cannot be 3", name)
		}
		return nil
	}
	tb = Dictionary{}.Int("b", 2, "attr b", intChecker)
	err = tb.Validate(map[string]interface{}{"b": 3})
	a.Error(err)
	a.True(strings.HasPrefix(err.Error(), "attribute b"))
	a.NoError(tb.Validate(map[string]interface{}{"b": 1}))

	floatChecker := func(f float32, name string) error {
		if f >= 0 && f <= 7 {
			return fmt.Errorf("attribute %s must not be between [0, 7]", name)
		}
		return nil
	}
	tb = Dictionary{}.Float("c", float32(-10.0), "attr c", floatChecker)
	err = tb.Validate(map[string]interface{}{"c": 5})
	a.Error(err)
	a.True(strings.HasPrefix(err.Error(), "attribute c"))
	a.NoError(tb.Validate(map[string]interface{}{"c": float32(-1.0)}))

	stringChecker := func(s string, name string) error {
		if !strings.HasPrefix(s, "valid") {
			return fmt.Errorf("attribute %s must have prefix valid", name)
		}
		return nil
	}
	tb = Dictionary{}.String("d", "valid.any_str", "attr d", stringChecker)
	err = tb.Validate(map[string]interface{}{"d": "invalid.str"})
	a.Error(err)
	a.True(strings.HasPrefix(err.Error(), "attribute d"))
	a.NoError(tb.Validate(map[string]interface{}{"d": "valid.str"}))

	intListChecker := func(intList []int, name string) error {
		if len(intList) != 2 {
			return fmt.Errorf("attribute %s must have length of 2", name)
		}
		return nil
	}
	tb = Dictionary{}.IntList("e", []int{1, 3}, "attr e", intListChecker)
	err = tb.Validate(map[string]interface{}{"e": []int{1, 2, 3}})
	a.Error(err)
	a.True(strings.HasPrefix(err.Error(), "attribute e"))
	a.NoError(tb.Validate(map[string]interface{}{"e": []int{4, 5}}))

	unknownChecker := func(i interface{}, name string) error {
		switch i.(type) {
		case int:
			return nil
		case string:
			return nil
		default:
			return fmt.Errorf("attribute %s must be of type int or string", name)
		}
	}
	tb = Dictionary{}.Unknown("f", "abc", "attr f", unknownChecker)
	err = tb.Validate(map[string]interface{}{"f": float32(4.5)})
	a.Error(err)
	a.True(strings.HasPrefix(err.Error(), "attribute f"))
	a.NoError(tb.Validate(map[string]interface{}{"f": 3}))
	a.NoError(tb.Validate(map[string]interface{}{"f": "any_str"}))

}

func TestParamsDocs(t *testing.T) {
	a := assert.New(t)

	a.Equal(11, len(PremadeModelParamsDocs))
	ExtractDocStringsOnce()
	a.Equal(20, len(PremadeModelParamsDocs))
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

	commonAttrs := Dictionary{"a": {Int, 1, "attribute a", nil}}
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
	tb := Dictionary{
		"a": {Int, 1, `this is a
multiple line
doc string.`, nil},
		"世界": {String, "", `42`, nil},
	}
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
