package sql

import (
	"database/sql"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
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
	if _, e := db.Exec(stmt); e != nil {
		return fmt.Errorf("sanityCheck failed executing %s: %q", stmt, e)
	}
	return nil
}

// According to https://stackoverflow.com/a/41263640/724872, we need
// gorm to run the DESCRIBE table command.
func describeTables(slct *extendedSelect, cfg *mysql.Config) (columnTypes, error) {
	db, e := gorm.Open("mysql", cfg.FormatDSN())
	if e != nil {
		return nil, e
	}
	defer db.Close()

	type Result struct {
		Field   string
		Type    string
		Null    string
		Key     string
		Default string
		Extra   string
	}

	ft := make(map[string]string)
	var dr Result
	for _, tn := range slct.tables {
		db.Raw("DESCRIBE " + tn + ";").Scan(&dr)
		ft[tn] = dr.Field
	}

	fmt.Println(ft) //debug
	return nil, nil
}
