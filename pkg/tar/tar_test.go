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

package tar

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const pythonFile = `
import sys

print(sys.version)
`
const readmeMdFile = `
# Test document
`

func TestTarUnTarDir(t *testing.T) {
	a := assert.New(t)
	testSubDir := "sub_dir"
	testDir := "sqlflow_tar"
	tarName := "test_tar.tar.gz"

	curDir, err := os.Getwd()
	a.NoError(err)
	defer os.Chdir(curDir)

	tmpDir, err := ioutil.TempDir("/tmp", "sqlflow_tar")
	a.NoError(err)
	defer os.RemoveAll(tmpDir)
	os.Chdir(tmpDir)
	err = os.Mkdir(testDir, 0755)
	a.NoError(err)
	// init directory with a sub directory and one file in each directory
	err = ioutil.WriteFile(fmt.Sprintf("%s/README.md", testDir),
		[]byte(readmeMdFile), 0644)
	a.NoError(err)
	err = os.Mkdir(fmt.Sprintf("%s/%s", testDir, testSubDir), 0755)
	a.NoError(err)
	err = ioutil.WriteFile(fmt.Sprintf("%s/%s/%s", testDir, testSubDir, "main.py"),
		[]byte(pythonFile), 0644)
	a.NoError(err)

	ZipDir(testDir, tarName)
	a.FileExists(tarName)

	err = UnzipDir(tarName, "out")
	a.NoError(err)
	a.DirExists("out/" + testDir)
	a.DirExists(fmt.Sprintf("out/%s/%s", testDir, testSubDir))

	buf, err := ioutil.ReadFile(fmt.Sprintf("out/%s/%s", testDir, "README.md"))
	a.NoError(err)
	a.Equal(readmeMdFile, string(buf))

	buf, err = ioutil.ReadFile(fmt.Sprintf("out/%s/%s/%s", testDir, testSubDir, "main.py"))
	a.NoError(err)
	a.Equal(pythonFile, string(buf))

}
