package sql

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/go-sql-driver/mysql"
)

func run(slct string, cfg *mysql.Config) error {
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		return e
	}
	defer db.Close()

	r, e := newParser().Parse(slct)
	if e != nil {
		return e
	}

	if r.train {
		cwd, e := ioutil.TempDir("/tmp", "sqlflow-training")
		if e != nil {
			return e
		}
		defer os.RemoveAll(cwd)

		fts, e := verify(r, db)
		if e != nil {
			return e
		}

		if e := train(r, fts, cfg, cwd); e != nil {
			return e
		}

		if e := save(db, r.save, cwd, slct); e != nil {
			return e
		}
	} else {
		inferParsed := r

		cwd, e := ioutil.TempDir("/tmp", "sqlflow-predicting")
		if e != nil {
			return e
		}
		defer os.RemoveAll(cwd)

		trainSlct, e := load(db, inferParsed.model, cwd)
		if e != nil {
			return e
		}

		trainParsed, e := newParser().Parse(trainSlct)
		if e != nil {
			return e
		}

		if e := verifyColumnNameAndType(trainParsed, inferParsed, db); e != nil {
			return e
		}

		if e := createPredictionTable(trainParsed, inferParsed, db); e != nil {
			return e
		}

		if e := infer(trainParsed, inferParsed, cfg, cwd); e != nil {
			return e
		}
	}

	return nil
}

func train(pr *extendedSelect, fts fieldTypes, cfg *mysql.Config, cwd string) (e error) {
	var program bytes.Buffer
	if e := generateTFProgram(&program, pr, fts, cfg); e != nil {
		return e
	}

	cmd := tensorflowCmd(cwd)
	cmd.Stdin = &program
	o, e := cmd.CombinedOutput()
	if e != nil {
		return e
	}
	if !strings.Contains(string(o), "Done training") {
		return fmt.Errorf(string(o) + "\nTraining failed")
	}

	return nil
}

// Create prediction table with appropriate column type.
// If prediction table already exists, it will be overwritten.
func createPredictionTable(trainParsed, inferParsed *extendedSelect, db *sql.DB) (e error) {
	if len(strings.Split(inferParsed.into, ".")) != 3 {
		return fmt.Errorf("invalid inferParsed.into %s. should be DBName.TableName.ColumnName", inferParsed.into)
	}
	tableName := strings.Join(strings.Split(inferParsed.into, ".")[:2], ".")
	columnName := strings.Split(inferParsed.into, ".")[2]

	dropStmt := fmt.Sprintf("drop table if exists %s;", tableName)
	if _, e := db.Query(dropStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", dropStmt, e)
	}

	fts, e := verify(trainParsed, db)
	if e != nil {
		return e
	}

	var b bytes.Buffer
	fmt.Fprintf(&b, "create table %s (", tableName)
	for _, c := range trainParsed.columns {
		typ, ok := fts.get(c.val)
		if !ok {
			return fmt.Errorf("createPredictionTable: Cannot find type of field %s", c.val)
		}
		fmt.Fprintf(&b, "%s %s, ", c.val, typ)
	}
	tpy, _ := fts.get(trainParsed.label)
	fmt.Fprintf(&b, "%s %s);", columnName, tpy)

	createStmt := b.String()
	if _, e := db.Query(createStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", createStmt, e)
	}

	return nil
}

func infer(trainParsed, inferParsed *extendedSelect, cfg *mysql.Config, cwd string) (e error) {
	return fmt.Errorf("infer not implemented")
}
