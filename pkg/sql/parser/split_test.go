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

package parser

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSplit(t *testing.T) {
	a := assert.New(t)

	//	{ // one SQL statement
	//		sql := `select 1;`
	//		s, err := split(sql)
	//		a.NoError(err)
	//		a.Equal(1, len(s))
	//		a.Equal(`select 1;`, s[0])
	//	}
	//
	//	{ // two SQL statements with comments
	//		sql := `-- this is a comment
	//select
	//1; select 2;`
	//		s, err := split(sql)
	//		a.NoError(err)
	//		a.Equal(2, len(s))
	//		a.Equal(`-- this is a comment
	//select
	//1;`, s[0])
	//		a.Equal(` select 2;`, s[1])
	//	}
	//
	//	{ // two SQL statements, the first one is extendedSQL
	//		sql := `select 1 to train; select 1;`
	//		s, err := split(sql)
	//		a.NoError(err)
	//		a.Equal(2, len(s))
	//		a.Equal(`select 1 to train;`, s[0])
	//		a.Equal(` select 1;`, s[1])
	//	}
	//
	//	{ // two SQL statements, the second one is extendedSQL
	//		sql := `select 1; select 1 to train;`
	//		s, err := split(sql)
	//		a.NoError(err)
	//		a.Equal(2, len(s))
	//		a.Equal(`select 1;`, s[0])
	//		a.Equal(` select 1 to train;`, s[1])
	//	}
	//
	//	{ // three SQL statements, the second one is extendedSQL
	//		sql := `select 1; select 1 to train; select 3;`
	//		s, err := split(sql)
	//		a.NoError(err)
	//		a.Equal(3, len(s))
	//		a.Equal(`select 1;`, s[0])
	//		a.Equal(` select 1 to train;`, s[1])
	//		a.Equal(` select 3;`, s[2])
	//	}

	{ // two SQL statements, the first standard SQL has an error
		sql := `seleeeeeect 1; select 1 to train;`
		s, err := split(sql)
		fmt.Println(err)
		a.Error(err)
		a.Equal(0, len(s))
	}
}
