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

package experimental

import (
	"strings"

	"sqlflow.org/sqlflow/go/parser"
)

// RewriteStatementsWithHints combines the hints into the standard SQL(s)
//
// FIXME(weiguoz): I'm not happy with such an implementation.
// I mean it is not clean that sqlflow handles such database relative details.
func rewriteStatementsWithHints(stmts []*parser.SQLFlowStmt, dialect string) []*parser.SQLFlowStmt {
	hints, sqls := splitHints(stmts, dialect)
	if len(hints) > 0 {
		for _, sql := range sqls {
			if !sql.IsExtendedSyntax() {
				sql.Original = hints + sql.Original
			}
		}
	}
	return sqls
}

func splitHints(stmts []*parser.SQLFlowStmt, dialect string) (string, []*parser.SQLFlowStmt) {
	hints, sqls := "", []*parser.SQLFlowStmt{}
	for _, stmt := range stmts {
		if isHint(stmt, dialect) {
			hints += stmt.Original + "\n" // alisa's requirements
		} else {
			sqls = append(sqls, stmt)
		}
	}
	return hints, sqls
}

func isHint(stmt *parser.SQLFlowStmt, dialect string) bool {
	if !stmt.IsExtendedSyntax() {
		if dialect == "alisa" {
			return isAlisaHint(stmt.Original)
		}
		// TODO(weiguoz) handle if submitter is "maxcompute" or "hive"
	}
	return false
}

func isAlisaHint(sql string) bool {
	for {
		sql = strings.TrimSpace(sql)
		// TODO(weiguoz): Let's remove the following code if we clean the comments before
		if strings.HasPrefix(sql, "--") {
			eol := strings.IndexAny(sql, "\n\r")
			if eol != -1 {
				sql = sql[eol+1:]
			} else {
				break
			}
		} else {
			break
		}
	}
	return strings.HasPrefix(strings.ToLower(sql), "set ")
}
