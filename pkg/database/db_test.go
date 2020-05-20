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

package database

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDatabaseParseURL(t *testing.T) {
	a := assert.New(t)
	driver, dataSource, e := ParseURL(GetTestingMySQLURL())
	a.EqualValues(driver, "mysql")
	a.EqualValues(dataSource, fmt.Sprintf("%s:%s@%s(%s)/?maxAllowedPacket=0",
		GetTestingMySQLConfig().User, GetTestingMySQLConfig().Passwd, GetTestingMySQLConfig().Net, GetTestingMySQLConfig().Addr))
	a.NoError(e)
}

func TestDatabaseDriverList(t *testing.T) {
	a := assert.New(t)
	expected := []string{"alisa", "hive", "maxcompute", "mysql"}
	a.EqualValues(expected, sql.Drivers())
}
