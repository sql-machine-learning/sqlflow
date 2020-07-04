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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckImageExists(t *testing.T) {
	a := assert.New(t)
	os.Setenv("SQLFLOW_MODEL_ZOO_REGISTRY", "hub.docker.com")
	exists := imageExistsOnRegistry("sqlflow/sqlflow", "latest")
	a.True(exists)
	exists = imageExistsOnRegistry("sqlflow/sqlflowxxxx", "latest")
	a.False(exists)
}

func TestBuildAndPush(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST_DB") != "mysql" {
		t.Skip("only test build and push to model zoo when SQLFLOW_TEST_DB=mysql")
	}
	a := assert.New(t)
	dir, err := mockTmpModelRepo()
	a.NoError(err)
	err = buildAndPushImage(dir, "sqlflow/mytest_model_image", "v0.1", true)
	a.NoError(err)
	// TODO(typhoonzero): add actual push tests
}

func TestBuildAndPushKaniko(t *testing.T) {
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		t.Skip("skip TestBuildAndPushKaniko when not in kubernetes cluster")
	}
	a := assert.New(t)
	dir, err := mockTmpModelRepo()
	a.NoError(err)
	err = buildAndPushImageKaniko(dir, "typhoon1986/mytest_model_image", "v0.1", false)
	a.NoError(err)
}
