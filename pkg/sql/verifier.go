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
	"bytes"
	"fmt"
	"strings"

	"sqlflow.org/sqlflow/pkg/parser"
)

// fieldTypes[field]type.
type fieldTypes map[string]string

func (ft fieldTypes) String() string {
	var b bytes.Buffer
	for field, typ := range ft {
		fmt.Fprintf(&b, "%s, %s\n", field, typ)
	}
	return b.String()
}

func (ft fieldTypes) get(ident string) (string, bool) {
	typ, ok := ft[ident]
	if !ok {
		return "", false
	}
	return typ, ok
}

// decomp returns the table name and field name in the given
// identifier: t.f=>(t,f), db.t.f=>(db.t,f), f=>("",f).
func decomp(ident string) (tbl string, fld string) {
	// Note: Hive driver represents field names in lower cases, so we convert all identifier
	// to lower case
	ident = strings.ToLower(ident)
	idx := strings.LastIndex(ident, ".")
	if idx == -1 {
		return "", ident
	}
	return ident[0:idx], ident[idx+1:]
}

// verify checks the standard SELECT part is syntactically and logically legal.
//
// It returns a fieldTypes describing types of fields in SELECT.
func verify(q string, db *DB) (fieldTypes, error) {
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("query %s gives 0 row", q)
	}

	if rows.Err() != nil {
		return nil, err
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	ft := make(fieldTypes)
	for _, ct := range columnTypes {
		_, fld := decomp(ct.Name())
		typeName := ct.DatabaseTypeName()
		if _, ok := ft[fld]; ok {
			return nil, fmt.Errorf("duplicated field name %s", fld)
		}
		ft[fld] = typeName
	}

	return ft, nil
}

// getColumnTypes is quiet like verify but accept a SQL string as input, and returns
// an ordered list of the field types.
func getColumnTypes(slct string, db *DB) ([]string, []string, error) {
	rows, err := db.Query(slct)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil, fmt.Errorf("query %s gives 0 row", slct)
	}

	if rows.Err() != nil {
		return nil, nil, err
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, nil, err
	}

	ft := []string{}
	flds := []string{}
	for _, ct := range columnTypes {
		_, fld := decomp(ct.Name())
		typeName := ct.DatabaseTypeName()
		flds = append(flds, fld)
		ft = append(ft, typeName)
	}

	return flds, ft, nil
}

// Check train and pred clause uses has the same feature columns
// 1. every column field in the training clause is selected in the pred clause, and they are of the same type
func verifyColumnNameAndType(trainParsed, predParsed *parser.SQLFlowSelectStmt, db *DB) error {
	trainFields, e := verify(trainParsed.StandardSelect.String(), db)
	if e != nil {
		return e
	}

	predFields, e := verify(predParsed.StandardSelect.String(), db)
	if e != nil {
		return e
	}

	for _, c := range trainParsed.Columns["feature_columns"] {
		name, err := getExpressionFieldName(c)
		if err != nil {
			return err
		}
		it, ok := predFields.get(name)
		if !ok {
			return fmt.Errorf("predFields doesn't contain column %s", name)
		}
		tt, _ := trainFields.get(name)
		if it != tt {
			return fmt.Errorf("field %s type dismatch %v(pred) vs %v(train)", name, it, tt)
		}
	}
	return nil
}
