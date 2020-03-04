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

package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSubmitter(t *testing.T) {
	a := assert.New(t)
	s1 := GetSubmitter("default")
	s2 := GetSubmitter("default")
	// call GetSubmitter should get 2 different objects
	a.False(s1 == s2)
	s3 := GetSubmitter("pai")
	_, ok := s3.(*paiSubmitter)
	a.True(ok)
}
