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
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	old "sqlflow.org/sqlflow/go/sql/testdata"
)

func TestDatabaseTestDataFeatureDerivationCaseMySQL(t *testing.T) {
	a := assert.New(t)
	// TODO(wangkuiyi): The +"\n" and removeBlankLines are all
	// trick to match without editing the old package.  We will
	// remove them once after the construction of the new testdata
	// package.
	a.Equal(removeBlankLines(old.FeatureDerivationCaseSQL+"\n"),
		FeatureDerivationCaseMySQL())
}

func TestDatabaseTestDataFeatureDerivationCaseHive(t *testing.T) {
	a := assert.New(t)
	// TODO(wangkuiyi): The +"\n" and removeBlankLines are all
	// trick to match without editing the old package.  We will
	// remove them once after the construction of the new testdata
	// package.
	a.Equal(removeBlankLines(old.FeatureDerivationCaseSQLHive+"\n"),
		FeatureDerivationCaseHive())

	writeFile("/sqlflow/a", removeBlankLines(old.FeatureDerivationCaseSQLHive+"\n"))
	writeFile("/sqlflow/b", FeatureDerivationCaseHive())
}

func writeFile(fn, content string) {
	ioutil.WriteFile(fn, []byte(content), 0644)
}
