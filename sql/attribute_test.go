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

func TestAttrs(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	s := statementWithAttrs("estimator.hidden_units = [10, 20]")
	r, e := parser.Parse(s)
	a.NoError(e)
	attrs, err := resolveAttribute(&r.trainAttrs)
	a.NoError(err)
	attr := attrs["estimator.hidden_units"]
	a.Equal("estimator", attr.Prefix)
	a.Equal("hidden_units", attr.Name)
	a.Equal([]interface{}([]interface{}{10, 20}), attr.Value)

	s = statementWithAttrs("dataset.name = hello")
	r, e = parser.Parse(s)
	a.NoError(e)
	attrs, err = resolveAttribute(&r.trainAttrs)
	a.NoError(err)
	attr = attrs["dataset.name"]
	a.Equal("dataset", attr.Prefix)
	a.Equal("name", attr.Name)
	a.Equal("hello", attr.Value)
}
