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

import "fmt"

// Works with MySQL and Hive.  MaxCompute doesn't have the concept of database.
func createDatabase(db string) string {
	return fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;\n", db)
}

func dropTableIfExists(tbl string) string {
	return fmt.Sprintf("DROP TABLE IF EXISTS %s;\n", tbl)
}

func createTable(tbl, schema string) string {
	return fmt.Sprintf("CREATE TABLE %s (%s);\n", tbl, schema)
}

func recreateTable(tbl, schema string) string {
	return dropTableIfExists(tbl) +
		createTable(tbl, schema)
}

// Works with MySQL and MaxCompute.
func insertData(tbl, data string) string {
	return fmt.Sprintf("INSERT INTO %s VALUES%s;\n", tbl, data)
}

// Works with Hive
func insertDataHive(tbl, data string) string {
	return fmt.Sprintf("INSERT INTO TABLE %s VALUES%s;\n", tbl, data)
}
