package sql

import (
	"database/sql"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

type columnTypes map[string]string

func verify(slct *extendedSelect, cfg *mysql.Config) (columnTypes, error) {
	if e := sanityCheck(slct, cfg); e != nil {
		return nil, e
	}
	// return describeTables(slct, db)
	return nil, nil
}

func sanityCheck(slct *extendedSelect, cfg *mysql.Config) error {
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		return fmt.Errorf("sanityCheck cannot connect to MySQL: %q", e)
	}
	defer db.Close()

	oldLimit := slct.standardSelect.limit
	defer func() {
		slct.standardSelect.limit = oldLimit
	}()

	slct.standardSelect.limit = "1"
	stmt := slct.standardSelect.String()
	if _, e := db.Query(stmt); e != nil {
		return fmt.Errorf("sanityCheck failed executing %s: %q", stmt, e)
	}
	return nil
}

/*
type fieldTypes map[string]map[string]string

func (ft fieldTypes) add(fld, tbl, typ string) {
	if m, ok := ft[fld]; !ok {
		ft[fld] = map[string]string{tbl: typ}
	} else {
		m[tbl] = typ
	}
}

func (ft fieldTypes) get(fld string) string {
	tbl := ""
	if splt := strings.Split(fld, "."); len(splt) > 1 {
		if len(splt) != 2 {
			log.Panicf("fieldTypes.get(fld=%s): more than one dots", fld)
		}
		tbl = splt[0]
		fld = splt[1]
	}
	return ""
}
*/
func describeTables(slct *extendedSelect, cfg *mysql.Config) (columnTypes, error) {
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		return nil, e
	}
	defer db.Close()

	var (
		Field   string
		Type    string
		Null    string
		Key     string
		Default string
		Extra   string
	)

	ft := make(map[string]string)
	for _, tn := range slct.tables {
		rows, e := db.Query("DESCRIBE " + tn)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			rows.Scan(&Field, &Type, &Null, &Key, &Default, &Extra)
			fmt.Println(Field, Type, Null, Key, Default, Extra)
		}
	}

	fmt.Println(ft) //debug
	return nil, nil
}
