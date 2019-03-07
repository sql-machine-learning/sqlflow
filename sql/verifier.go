package sql

import (
	"fmt"
	"strings"
)

// fieldTypes[field][table]type.  For more information, please check
// verifier_test.go.
type fieldTypes map[string]map[string]string

// verify checks the following:
//
// 1. The standard SELECT part is syntactically and logically legal.
//
// 2. TODO(yi): The COLUMN clause refers to only fields in the SELECT
//    clause.  Please be aware that both SELECT and COLUMN might have
//    star '*'.
//
// It returns a fieldTypes describing types of fields in SELECT.
func verify(slct *extendedSelect, db *DB) (ft fieldTypes, e error) {
	if e := dryRunSelect(slct, db); e != nil {
		return nil, e
	}
	return describeTables(slct, db)
}

func dryRunSelect(slct *extendedSelect, db *DB) error {
	oldLimit := slct.standardSelect.limit
	defer func() {
		slct.standardSelect.limit = oldLimit
	}()

	slct.standardSelect.limit = "1"
	stmt := slct.standardSelect.String()
	rows, e := db.Query(stmt)
	if e != nil {
		return fmt.Errorf("dryRunSelect failed executing %s: %q", stmt, e)
	}
	defer rows.Close()

	return rows.Err()
}

func (ft fieldTypes) get(ident string) (string, bool) {
	tbl, fld := decomp(ident)
	tbls, ok := ft[fld]
	if !ok {
		return "", false
	}
	if len(tbl) == 0 && len(tbls) == 1 {
		for _, typ := range tbls {
			return typ, true
		}
	}
	typ, ok := tbls[tbl]
	return typ, ok
}

// decomp returns the table name and field name in the given
// identifier: t.f=>(t,f), db.t.f=>(db.t,f), f=>("",f).
func decomp(ident string) (tbl string, fld string) {
	s := strings.Split(ident, ".")
	return strings.Join(s[:len(s)-1], "."), s[len(s)-1]
}

// Retrieve the type of fields mentioned in SELECT.
func describeTables(slct *extendedSelect, db *DB) (ft fieldTypes, e error) {
	ft = indexSelectFields(slct)
	hasStar := len(ft) == 0
	for _, tn := range slct.tables {
		slct := "SELECT * from " + tn + " limit 1"
		rows, err := db.Query(slct)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		cols, err := rows.Columns()
		if err != nil {
			return nil, err
		}

		if !rows.Next() {
			return nil, fmt.Errorf("table is Empty. table name: %s", tn)
		}

		if rows.Err() != nil {
			return nil, e
		}

		columnTypes, err := rows.ColumnTypes()
		if err != nil {
			return nil, err
		}
		for i, ct := range columnTypes {
			fld := cols[i]
			typeName := ct.DatabaseTypeName()

			if hasStar {
				if _, ok := ft[fld]; !ok {
					ft[fld] = make(map[string]string)
				}
				ft[fld][tn] = typeName
			} else {
				if tbls, ok := ft[fld]; ok {
					if len(tbls) == 0 {
						tbls[tn] = typeName
					} else if _, ok := tbls[tn]; ok {
						tbls[tn] = typeName
					}
				}
			}
		}

	}
	return ft, nil
}

// Index fields in the SELECT clause.  For `SELECT f`, returns {f:{}}.
// For `SELECT t.f`, returns {f:{t:1}}.  For `SELECT t1.f, t2.f`,
// returns {f:{t1:1,t2:1}}.  For `SELECT ... * ...`, returns {}.
func indexSelectFields(slct *extendedSelect) (ft fieldTypes) {
	ft = make(fieldTypes)
	for _, f := range slct.fields {
		if f == "*" {
			return fieldTypes{}
		}
		tbl, fld := decomp(f)
		if _, ok := ft[fld]; !ok {
			ft[fld] = make(map[string]string)
		}
		if len(tbl) > 0 {
			ft[fld][tbl] = ""
		}
	}
	return ft
}

// Check train and pred clause uses has the same feature columns
// 1. every column field in the training clause is selected in the pred clause, and they are of the same type
func verifyColumnNameAndType(trainParsed, predParsed *extendedSelect, db *DB) error {
	trainFields, e := verify(trainParsed, db)
	if e != nil {
		return e
	}

	predFields, e := verify(predParsed, db)
	if e != nil {
		return e
	}

	for _, c := range trainParsed.columns {
		it, ok := predFields.get(c.val)
		if !ok {
			return fmt.Errorf("predFields doesn't contain column %s", c.val)
		}
		tt, _ := trainFields.get(c.val)
		if it != tt {
			return fmt.Errorf("field %s type dismatch %s(pred) vs %s(train)", c.val, it, tt)
		}
	}
	return nil
}
