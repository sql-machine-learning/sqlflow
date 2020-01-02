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

package sqlfs

import (
	"database/sql"
	"io"

	pb "sqlflow.org/sqlflow/pkg/proto"
)

const bufSize = 32 * 1024

// Create creates a new table or truncates an existing table and
// returns a writer.
func Create(db *sql.DB, dbms, table string, session *pb.Session) (io.WriteCloser, error) {
	if dbms == "hive" {
		return newHiveWriter(db, session.HiveLocation, table, session.HdfsUser, session.HdfsPass, session.HdfsNamenodeAddr, bufSize)
	}
	return newSQLWriter(db, dbms, table, bufSize)
}
