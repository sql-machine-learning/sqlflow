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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateAttribute(t *testing.T) {
	a := assert.New(t)
	attrs := make(map[string]*attribute)
	attrs["train.steps"] = &attribute{FullName: "train.steps", Name: "steps", Prefix: "train", Value: "100"}
	attrs["model.hidden_units"] = &attribute{FullName: "model.hidden_units", Name: "hidden_units", Prefix: "model", Value: "[100,200,300]"}
	attrs["model.learning_rate"] = &attribute{FullName: "model.learning_rate", Name: "learning_rate", Prefix: "model", Value: "0.01"}
	attrs["engine.type"] = &attribute{FullName: "engine.type", Name: "type", Prefix: "engine", Value: "yarn"}
	err := ValidateAttributes(attrs)
	a.Nil(err)

	for k := range attrs {
		delete(attrs, k)
	}
	attrs["new.x"] = &attribute{FullName: "new.x", Name: "x", Prefix: "new", Value: "0.01"}
	err = ValidateAttributes(attrs)
	a.NotNil(err)

	for k := range attrs {
		delete(attrs, k)
	}
	attrs["engine.type"] = &attribute{FullName: "engine.type", Name: "type", Prefix: "engine", Value: "&#HNC#&="}
	err = ValidateAttributes(attrs)
	a.Nil(err)
}
