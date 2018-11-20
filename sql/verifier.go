package sql

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
)

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
func verify(slct *extendedSelect, cfg *mysql.Config) (fieldTypes, error) {
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		return nil, fmt.Errorf("verify cannot connect to MySQL: %q", e)
	}
	defer db.Close()

	if e := dryRunSelect(slct, db); e != nil {
		return nil, e
	}
	return describeTables(slct, db)
}

func dryRunSelect(slct *extendedSelect, db *sql.DB) error {
	oldLimit := slct.standardSelect.limit
	defer func() {
		slct.standardSelect.limit = oldLimit
	}()

	slct.standardSelect.limit = "1"
	stmt := slct.standardSelect.String()
	if _, e := db.Query(stmt); e != nil {
		return fmt.Errorf("dryRunSelect failed executing %s: %q", stmt, e)
	}
	return nil
}

func (ft fieldTypes) get(ident string) (string, bool) {
	tbl, fld := decomp(ident)
	tbls, ok := ft[fld]
	if !ok {
		return "", false
	}
	typ, ok := tbls[tbl]
	if !ok {
		return "", false
	}
	return typ, true
}

// decomp returns the table name and field name in the given
// identifier: t.f=>(t,f), db.t.f=>(db.t,f), f=>("",f).
func decomp(ident string) (tbl string, fld string) {
	s := strings.Split(ident, ".")
	return strings.Join(s[:len(s)-1], "."), s[len(s)-1]
}

// Retrieve the type of fields mentioned in SELECT.
func describeTables(slct *extendedSelect, db *sql.DB) (fieldTypes, error) {
	fields := indexSelectFields(slct)
	hasStar := len(fields) == 0
	for _, tn := range slct.tables {
		rows, e := db.Query("DESCRIBE " + tn)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var fld, typ, null, key, deflt, extra string
			rows.Scan(&fld, &typ, &null, &key, &deflt, &extra)

			if hasStar {
				if _, ok := fields[fld]; !ok {
					fields[fld] = make(map[string]string)
				}
				fields[fld][tn] = typ
			} else {
				if tbls, ok := fields[fld]; ok {
					if len(tbls) == 0 {
						tbls[tn] = typ
					} else if _, ok := tbls[tn]; ok {
						tbls[tn] = typ
					}
				}
			}
		}
	}
	return fields, nil
}

// Index fields in the SELECT clause.  For `SELECT f`, returns {f:{}}.
// For `SELECT t.f`, returns {f:{t:1}}.  For `SELECT t1.f, t2.f`,
// returns {f:{t1:1,t2:1}}.  For `SELECT ... * ...`, returns {}.
func indexSelectFields(slct *extendedSelect) fieldTypes {
	fields := make(fieldTypes)
	for _, f := range slct.fields {
		if f == "*" {
			return fieldTypes{}
		}
		tbl, fld := decomp(f)
		if _, ok := fields[fld]; !ok {
			fields[fld] = make(map[string]string)
		}
		if len(tbl) > 0 {
			fields[fld][tbl] = ""
		}
	}
	return fields
}
