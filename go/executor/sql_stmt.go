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

package executor

import (
	"database/sql"
	"fmt"
	"strings"

	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/pipe"
)

func runNormalStmt(wr *pipe.Writer, slct string, db *database.DB) error {
	if isQuery(slct) {
		return runQuery(wr, slct, db)
	}
	return runExec(wr, slct, db)
}

// TODO(weiguo): isQuery is a hacky way to decide which API to call:
// https://golang.org/pkg/database/sql/#DB.Exec .
// We will need to extend our parser to be a full SQL parser in the future.
func isQuery(slct string) bool {
	s := strings.ToUpper(strings.TrimSpace(slct))
	has := strings.Contains
	// remove comment lines
	if strings.HasPrefix(s, "-- ") {
		lines := strings.Split(s, "\n")
		noCommentLines := []string{}
		for _, l := range lines {
			if !strings.HasPrefix(l, "-- ") {
				noCommentLines = append(noCommentLines, l)
			}
		}
		s = strings.Join(noCommentLines, "\n")
	}
	if strings.HasPrefix(s, "SELECT") && !has(s, "INTO") {
		return true
	}
	if strings.HasPrefix(s, "SHOW") && (has(s, "CREATE") || has(s, "DATABASES") || has(s, "TABLES")) {
		return true
	}
	if strings.HasPrefix(s, "DESC") || strings.HasPrefix(s, "EXPLAIN") {
		return true
	}
	return false
}

// query runs slct and writes the retrieved rows into pipe wr.
func runQuery(wr *pipe.Writer, slct string, db *database.DB) error {
	rows, err := db.Query(slct)
	if err != nil {
		return fmt.Errorf("runQuery failed: %v", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %v", err)
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return fmt.Errorf("failed to get columnTypes: %v", err)
	}
	header := make(map[string]interface{})
	header["columnNames"] = columns
	if e := wr.Write(header); e != nil {
		return e
	}
	for rows.Next() {
		if e := parseRow(columns, columnTypes, rows, wr); e != nil {
			return e
		}
	}
	return nil
}

// parseRow calls rows.Scan to retrieve the current row, and convert
// each cell value from {}interface to an accuracy value.  It then
// writes the converted row into wr.
func parseRow(columns []string, columnTypes []*sql.ColumnType, rows *sql.Rows, wr *pipe.Writer) error {
	// Since we don't know the table schema in advance, we create
	// a slice of empty interface and add column types at
	// runtime. Some databases support dynamic types between rows,
	// such as sqlite's affinity. So we move columnTypes inside
	// the row.Next() loop.
	count := len(columns)
	values := make([]interface{}, count)
	for i, ct := range columnTypes {
		// NOTE(typhoonzero): Hive TIMESTAMP_TYPE column will return string value, but ct.ScanType() returns int64
		// https://github.com/sql-machine-learning/sqlflow/issues/1256
		if ct.DatabaseTypeName() == "TIMESTAMP_TYPE" {
			values[i] = new(string)
			continue
		}
		values[i] = newZeroValue(ct.ScanType())
	}

	if err := rows.Scan(values...); err != nil {
		return err
	}

	row := make([]interface{}, count)
	for i, val := range values {
		v, e := fieldValue(val)
		if e != nil {
			return e
		}
		row[i] = v
	}
	if e := wr.Write(row); e != nil {
		return e
	}
	return nil
}

func runExec(wr *pipe.Writer, slct string, db *database.DB) error {
	res, e := db.Exec(slct)
	if e != nil {
		return fmt.Errorf("runExec failed: %v", e)
	}
	affected, e := res.RowsAffected()
	if e != nil {
		return fmt.Errorf("failed to get affected row number: %v", e)
	}
	if affected > 1 {
		return wr.Write(fmt.Sprintf("%d rows affected", affected))
	}
	// gomaxcompute does not return affected rows number
	if affected < 0 {
		return wr.Write("OK")
	}
	return wr.Write(fmt.Sprintf("%d row affected", affected))
}

// -------------------------- utilities --------------------------------------

func parseTableColumn(s string) (string, string, error) {
	pos := strings.LastIndex(s, ".")
	if pos == -1 || pos == len(s)-1 {
		return "", "", fmt.Errorf("can not separate %s to table and column", s)
	}
	return s[:pos], s[pos+1:], nil
}
