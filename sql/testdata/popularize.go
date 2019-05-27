package testdata

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

import (
	"strings"

	"github.com/sql-machine-learning/sqlflow/sql"
)

// Popularize reads SQL statements from the file named *.sql
// and runs each SQL statement with db.
func Popularize(db *sql.DB, sqlcontent string) error {
	// TODO(typhoonzero): Should consider .sql files like VALUES "a;b;c";
	sqlQueries := strings.Split(sqlcontent, ";")
	for _, sql := range sqlQueries {
		trimedSQL := strings.Trim(sql, " \n")
		if trimedSQL == "" {
			continue
		}
		_, e := db.Exec(trimedSQL)
		if e != nil {
			return e
		}
	}
	return nil
}
