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
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPartials(t *testing.T) {
	a := assert.New(t)
	tmpMap := make(map[string][]string)
	filler := &xgboostFiller{}

	partial := strPartial("obj",  func(r *xgboostFiller) *string { return &(r.Objective) })
	tmpMap["obj"] = []string{"binary:logistic"}
	e := partial(&tmpMap, filler)
	a.NoError(e)
	a.Equal(filler.Objective, "binary:logistic")
	_, ok := tmpMap["obj"]
	a.Equal(ok, false)
	// duplicate attr setting
	tmpMap["obj"] = []string{"binary:logistic"}
	e = partial(&tmpMap, filler)
	a.Error(e)
	// len(val) > 1
	tmpMap["obj"] = []string{"binary:logistic", "reg:linear"}
	e = partial(&tmpMap, filler)
	a.Error(e)
	//
	tmpMap["obj"] = []string{"reg:linear"}
	filler.Objective = ""
	e = partial(&tmpMap, filler)
	a.NoError(e)
	a.Equal(filler.Objective, "reg:linear")
}
