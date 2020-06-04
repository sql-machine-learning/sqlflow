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
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTarUntar(t *testing.T) {
	a := assert.New(t)
	dir, err := ioutil.TempDir("/tmp", "tmp-sqlflow-repo")
	a.NoError(err)
	defer os.RemoveAll(dir)

	modelRepoDir := fmt.Sprintf("%s/my_test_models", dir)
	err = os.Mkdir(modelRepoDir, os.ModeDir)

	err = ioutil.WriteFile(
		fmt.Sprintf("%s/Dockerfile", dir),
		[]byte(sampleDockerfile), 0644)
	a.NoError(err)

	err = ioutil.WriteFile(
		fmt.Sprintf("%s/my_test_model.py", modelRepoDir),
		[]byte(sampleModelCode), 0644)
	a.NoError(err)
	err = ioutil.WriteFile(
		fmt.Sprintf("%s/__init__.py", modelRepoDir),
		[]byte(sampleInitCode), 0644)
	a.NoError(err)
	err = tarGzDir(dir, "mytar.tar.gz")
	a.NoError(err)

	err = untarGzDir("mytar.tar.gz", ".")
	if err != nil {
		a.FailNow("%v", err)
	}
	content, err := ioutil.ReadFile(fmt.Sprintf(".%s/my_test_model.py", modelRepoDir))
	if err != nil {
		a.FailNow("%v", err)
	}
	descs, err := getModelClasses(fmt.Sprintf(".%s", dir))
	a.NoError(err)
	a.Equal(1, len(descs))
	a.Equal("DNNClassifier", descs[0].Name)

	a.Equal(sampleModelCode, string(content))
	os.Remove("mytar.tar.gz")
	os.RemoveAll("./tmp")
}
