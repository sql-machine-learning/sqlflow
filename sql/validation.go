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

package sql

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
)

const (
	temporaryTableLifecycle = 14 // day(s)
	randomColumn            = "sqlflow_rdm"
	randomTablePrefix       = "sqlflow_tv_" // 'tv' = training & validation
)

var errNotSupportYet = errors.New("not support yet")

// SQLFlow generates a temporary table, + sqlflow_randowm column
func tableWithRandomColumn(db *DB, slct string) (string, error) {
	switch db.driverName {
	case "maxcompute":
		return createMaxcomputeRandomTable(db, slct)
	// TODO(weiguo): support other databases
	case "hive", "mysql", "sqlite3":
		return "", errNotSupportYet
	default:
		return "", fmt.Errorf("sqlflow currently doesn't support validation for %s", db.driverName)
	}
}

func createMaxcomputeRandomTable(db *DB, slct string) (string, error) {
	tbl := randomTableName()
	createStmt := fmt.Sprintf("CREATE TABLE %s LIFECYCLE %d AS SELECT *, RAND() AS %s FROM (%s) AS %s_ori", tbl, temporaryTableLifecycle, randomColumn, slct, tbl)
	if _, e := db.Exec(createStmt); e != nil {
		log.Errorf("create temporary table failed, statememnt:[%s], err:%v", createStmt, e)
		return "", e
	}
	return tbl, nil
}

func randomTableName() string {
	n := time.Now().UnixNano() / 1e3
	return fmt.Sprintf("%s%d", randomTablePrefix, n)
}
