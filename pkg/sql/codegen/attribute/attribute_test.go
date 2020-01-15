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
	"fmt"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestDictionaryValidate(t *testing.T) {
	a := assert.New(t)

	checker := func(i interface{}) error {
		ii, ok := i.(int)
		if !ok {
			return fmt.Errorf("%T %v should of type integer", i, i)
		}
		if ii < 0 {
			return fmt.Errorf("some error")
		}
		return nil
	}
	tb := Dictionary{"a": {Int, 1, "attribute a", checker}}
	a.NoError(tb.Validate(map[string]interface{}{"a": 1}))
	a.EqualError(tb.Validate(map[string]interface{}{"a": -1}), "some error")
	a.EqualError(tb.Validate(map[string]interface{}{"_a": -1}), fmt.Sprintf(errUnsupportedAttribute, "_a"))
	a.EqualError(tb.Validate(map[string]interface{}{"a": 1.0}), fmt.Sprintf(errUnexpectedType, "a", "int", 1.))
}

func TestPremadeModelParamsDocs(t *testing.T) {
	a := assert.New(t)

	a.Equal(18, len(PremadeModelParamsDocs))
	a.Equal(len(PremadeModelParamsDocs["DNNClassifier"]), 12)
	a.NotContains(PremadeModelParamsDocs["DNNClassifier"], "feature_columns")
	a.Contains(PremadeModelParamsDocs["DNNClassifier"], "optimizer")
	a.Contains(PremadeModelParamsDocs["DNNClassifier"], "hidden_units")
	a.Contains(PremadeModelParamsDocs["DNNClassifier"], "n_classes")

	a.True(reflect.DeepEqual(PremadeModelParamsDocs["xgboost.gbtree"], PremadeModelParamsDocs["xgboost.dart"]))
	a.True(reflect.DeepEqual(PremadeModelParamsDocs["xgboost.dart"], PremadeModelParamsDocs["xgboost.gblinear"]))
	a.Equal(23, len(PremadeModelParamsDocs["xgboost.gbtree"]))
	a.NotContains(PremadeModelParamsDocs["DNNClassifier"], "booster")

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
