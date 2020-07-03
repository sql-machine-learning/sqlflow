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

package testdata

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	old "sqlflow.org/sqlflow/go/sql/testdata"
)

func TestDatabaseTestDataIrisMySQL(t *testing.T) {
	a := assert.New(t)
	// TODO(wangkuiyi): The +"\n" and removeBlankLines are all
	// trick to match without editing the old package.  We will
	// remove them once after the construction of the new testdata
	// package.
	a.Equal(removeBlankLines(old.IrisSQL+"\n"),
		IrisMySQL("iris"))
}

func TestDatabaseTestDataIrisHive(t *testing.T) {
	a := assert.New(t)
	a.Equal(removeBlankLines(old.IrisHiveSQL+"\n"),
		IrisHive("iris"))
}

func TestDatabaseTestDataIrisMaxCompute(t *testing.T) {
	a := assert.New(t)
	a.Equal(removeBlankLines(old.IrisMaxComputeSQL+"\n"),
		IrisMaxCompute(`%[1]s`))
}

func removeBlankLines(s string) string {
	regex, err := regexp.Compile("\n\n")
	if err != nil {
		return ""
	}
	return regex.ReplaceAllString(s, "\n")
}
