package sql

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/go-sql-driver/mysql"
)

type columnTypes map[string]string

// verify checks the following:
//
// 1. The standard SELECT part is syntactically and logically legal.
//
// 2. The COLUMN clause refers to only fields in the SELECT clause.
//    Please be aware that both SELECT and COLUMN might have star '*'.
//
// It returns a map[string][string] from fields in COLUMN clause to SQL types.
func verify(slct *extendedSelect, cfg *mysql.Config) (columnTypes, error) {
	if e := checkSelect(slct, cfg); e != nil {
		return nil, e
	}
	// return describeTables(slct, db)
	return nil, nil
}

func checkSelect(slct *extendedSelect, cfg *mysql.Config) error {
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		return fmt.Errorf("checkSelect cannot connect to MySQL: %q", e)
	}
	defer db.Close()

	oldLimit := slct.standardSelect.limit
	defer func() {
		slct.standardSelect.limit = oldLimit
	}()

	slct.standardSelect.limit = "1"
	stmt := slct.standardSelect.String()
	if _, e := db.Query(stmt); e != nil {
		return fmt.Errorf("checkSelect failed executing %s: %q", stmt, e)
	}
	return nil
}

type fieldTypes map[string]map[string]string

func (ft fieldTypes) add(fld, tbl, typ string) {
	if m, ok := ft[fld]; !ok {
		ft[fld] = map[string]string{tbl: typ}
	} else {
		m[tbl] = typ
	}
}

func (ft fieldTypes) get(ident string) (string, bool) {
	var tbl, fld string
	if splt := strings.Split(ident, "."); len(splt) > 1 {
		if len(splt) != 2 {
			log.Panicf("fieldTypes.get(fld=%s): more than one dots", fld)
		}
		tbl = strings.Join(splt[0:len(splt)-1], ".")
		fld = splt[len(splt)-1]
	} else {
		tbl = ""
		fld = ident
	}

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

func describeTables(slct *extendedSelect, cfg *mysql.Config) (fieldTypes, error) {
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		return nil, e
	}
	defer db.Close()

	var (
		field string
		typ   string
		null  string
		key   string
		deflt string
		extra string
	)
	ft := make(fieldTypes)
	for _, tn := range slct.tables {
		rows, e := db.Query("DESCRIBE " + tn)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			rows.Scan(&field, &typ, &null, &key, &deflt, &extra)
			ft.add(field, tn, typ)
		}
	}
	return ft, nil
}
