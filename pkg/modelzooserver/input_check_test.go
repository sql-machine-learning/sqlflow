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

package modelzooserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInputCheck(t *testing.T) {
	a := assert.New(t)
	err := checkName("a")
	a.Error(err)
	err = checkName("aaaxxx and 1=1")
	a.Error(err)
	err = checkName("a_sample_model")
	a.NoError(err)

	err = checkImageURL("aa")
	a.Error(err)
	err = checkImageURL(`aaaxxx and 1=1`)
	a.Error(err)
	err = checkImageURL("hub.docker.com/pb/ccc")
	a.NoError(err)
	err = checkImageURL("my_published_model")
	a.NoError(err)

	err = checkTag("v0.0.1")
	a.NoError(err)
	err = checkTag("latest")
	a.NoError(err)
	err = checkTag("")
	a.NoError(err)
	err = checkTag("v1.0 and 1=1")
	a.Error(err)
}
