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

package verifier

import (
	"bytes"
	"fmt"
	"strings"

	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/parser"
)

// FieldTypes type records a mapping from field name to field type name.
type FieldTypes map[string]string

func (ft FieldTypes) String() string {
	var b bytes.Buffer
	for field, typ := range ft {
		fmt.Fprintf(&b, "%s, %s\n", field, typ)
	}
	return b.String()
}

// Get the field type.
func (ft FieldTypes) Get(ident string) (string, bool) {
	typ, ok := ft[ident]
	if !ok {
		return "", false
	}
	return typ, ok
}

// Decomp returns the table name and field name in the given
// identifier: t.f=>(t,f), db.t.f=>(db.t,f), f=>("",f).
func Decomp(ident string) (tbl string, fld string) {
	// Note: Hive driver represents field names in lower cases, so we convert all identifier
	// to lower case
	ident = strings.ToLower(ident)
	idx := strings.LastIndex(ident, ".")
	if idx == -1 {
		return "", ident
	}
	return ident[0:idx], ident[idx+1:]
}

// Verify checks the standard SELECT part is syntactically and logically legal.
//
// It returns a FieldTypes describing types of fields in SELECT.
func Verify(q string, db *database.DB) (FieldTypes, error) {
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

	ft := make(FieldTypes)
	for _, ct := range columnTypes {
		_, fld := Decomp(ct.Name())
		typeName := ct.DatabaseTypeName()
		if _, ok := ft[fld]; ok {
			return nil, fmt.Errorf("duplicated field name %s", fld)
		}
		ft[fld] = typeName
	}

	return ft, nil
}

func getExpressionFieldName(expr *parser.Expr) (string, error) {
	if expr.Type != 0 {
		return expr.Value, nil
	}
	if len(expr.Sexp) < 2 {
		return "", fmt.Errorf("error column clause format: %s, expected FEATURE_COLUMN(key, ...)", expr.Sexp)
	}
	fcNameExpr := expr.Sexp[1]
	return fcNameExpr.Value, nil
}

// VerifyColumnNameAndType check train and pred clause uses has the same feature columns
// 1. every column field in the training clause is selected in the pred clause, and they are of the same type
func VerifyColumnNameAndType(trainParsed, predParsed *parser.SQLFlowSelectStmt, db *database.DB) error {
	trainFields, e := Verify(trainParsed.StandardSelect.String(), db)
	if e != nil {
		return e
	}

	predFields, e := Verify(predParsed.StandardSelect.String(), db)
	if e != nil {
		return e
	}

	for _, c := range trainParsed.Columns["feature_columns"] {
		name, err := getExpressionFieldName(c)
		if err != nil {
			return err
		}
		it, ok := predFields.Get(name)
		if !ok {
			return fmt.Errorf("predFields doesn't contain column %s", name)
		}
		tt, _ := trainFields.Get(name)
		if it != tt {
			return fmt.Errorf("field %s type dismatch %v(pred) vs %v(train)", name, it, tt)
		}
	}
	return nil
}
