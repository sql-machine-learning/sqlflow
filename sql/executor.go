package sql

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/go-sql-driver/mysql"
)

func run(slct string, cfg *mysql.Config) error {
	pr, e := newParser().Parse(slct)
	if e != nil {
		return e
	}

	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		return e
	}
	defer db.Close()

	cwd, e := ioutil.TempDir("/tmp", "sqlflow")
	if e != nil {
		return e
	}
	defer os.RemoveAll(cwd)

	if pr.train {
		return train(pr, slct, db, cfg, cwd)
	}
	return infer(pr, db, cfg, cwd)
}

func train(pr *extendedSelect, slct string, db *sql.DB, cfg *mysql.Config, cwd string) error {
	fts, e := verify(pr, db)
	if e != nil {
		return e
	}

	var program bytes.Buffer
	if e := genTF(&program, pr, fts, cfg); e != nil {
		return e
	}

	cmd := tensorflowCmd(cwd)
	cmd.Stdin = &program
	o, e := cmd.CombinedOutput()
	if e != nil || !strings.Contains(string(o), "Done training") {
		return fmt.Errorf("Training failed %v: \n%s", e, o)
	}

	return save(db, pr.save, cwd, slct)
}

// Create prediction table with appropriate column type.
// If prediction table already exists, it will be overwritten.
func createPredictionTable(trainParsed, inferParsed *extendedSelect, db *sql.DB) error {
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

func infer(ir *extendedSelect, db *sql.DB, cfg *mysql.Config, cwd string) (e error) {
	trainSlct, e := load(db, ir.model, cwd)
	if e != nil {
		return e
	}

	// Parse the training SELECT statement used to train
	// the model for the prediction.
	tr, e := newParser().Parse(trainSlct)
	if e != nil {
		return e
	}

	if e := verifyColumnNameAndType(tr, ir, db); e != nil {
		return e
	}

	if e := createPredictionTable(tr, ir, db); e != nil {
		return e
	}

	ir.trainClause = tr.trainClause
	fts, e := verify(ir, db)

	var buf bytes.Buffer
	if e := genTF(&buf, ir, fts, cfg); e != nil {
		return e
	}

	// log.Println(string(buf.Bytes()))
	cmd := tensorflowCmd(cwd)
	cmd.Stdin = &buf
	o, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	log.Println(string(o))

	return fmt.Errorf("infer still under construction")
}
