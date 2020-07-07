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

package pai

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCheckpointDir(t *testing.T) {
	a := assert.New(t)
	os.Setenv("SQLFLOW_OSS_CHECKPOINT_DIR", "{\"host\": \"h.com\", \"arn\": \"acs:ram::9527:role\"}")
	defer os.Unsetenv("SQLFLOW_OSS_CHECKPOINT_DIR")
	ossModelPath, project := "", "pr0j"

	ckpoint, err := getCheckpointDir(ossModelPath, project)
	a.NoError(err)
	expectedCkp := fmt.Sprintf("oss://%s/?role_arn=acs:ram::9527:role/pai2oss_pr0j&host=h.com", BucketName)
	a.Equal(expectedCkp, ckpoint)
}
