package sql

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/go-sql-driver/mysql"
)

func Run(slct string, cfg *mysql.Config) (string, error) {
	pr, e := newParser().Parse(slct)
	if e != nil {
		return "Invalid SQL", e
	}

	if pr.extended {
		if err := runExtendedSQL(slct, cfg, pr); err != nil {
			return "", err
		}
		return "Job success", nil
	}
	return runStandardSQL(slct, cfg)
}

func runStandardSQL(slct string, cfg *mysql.Config) (string, error) {
	cmd := exec.Command("docker", "exec", "-t",
		// set password as envirnment variable to surpress warnings
		// https://stackoverflow.com/a/24188878/6794675
		"-e", fmt.Sprintf("MYSQL_PWD=%s", cfg.Passwd),
		"sqlflowtest",
		"mysql", fmt.Sprintf("-u%s", cfg.User),
		"-e", fmt.Sprintf("%s", slct))
	o, e := cmd.CombinedOutput()
	if e != nil {
		return "", fmt.Errorf("runStandardSQL failed %v: \n%s", e, o)
	}
	return string(o), nil
}

func runExtendedSQL(slct string, cfg *mysql.Config, pr *extendedSelect) error {
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
	return pred(pr, db, cfg, cwd)
}

func train(tr *extendedSelect, slct string, db *sql.DB, cfg *mysql.Config, cwd string) error {
	fts, e := verify(tr, db)
	if e != nil {
		return e
	}

	var program bytes.Buffer
	if e := genTF(&program, tr, fts, cfg); e != nil {
		return e
	}

	cmd := tensorflowCmd(cwd)
	cmd.Stdin = &program
	o, e := cmd.CombinedOutput()
	if e != nil || !strings.Contains(string(o), "Done training") {
		return fmt.Errorf("Training failed %v: \n%s", e, o)
	}

	m := model{workDir: cwd, TrainSelect: slct}
	return m.save(db, tr.save)
}

// Create prediction table with appropriate column type.
// If prediction table already exists, it will be overwritten.
func createPredictionTable(trainParsed, predParsed *extendedSelect, db *sql.DB) error {
	if len(strings.Split(predParsed.into, ".")) != 3 {
		return fmt.Errorf("invalid predParsed.into %s. should be DBName.TableName.ColumnName", predParsed.into)
	}
	tableName := strings.Join(strings.Split(predParsed.into, ".")[:2], ".")
	columnName := strings.Split(predParsed.into, ".")[2]

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

func pred(pr *extendedSelect, db *sql.DB, cfg *mysql.Config, cwd string) error {
	m, e := load(db, pr.model, cwd)
	if e != nil {
		return e
	}

	// Parse the training SELECT statement used to train
	// the model for the prediction.
	tr, e := newParser().Parse(m.TrainSelect)
	if e != nil {
		return e
	}

	if e := verifyColumnNameAndType(tr, pr, db); e != nil {
		return e
	}

	if e := createPredictionTable(tr, pr, db); e != nil {
		return e
	}

	pr.trainClause = tr.trainClause
	fts, e := verify(pr, db)

	var buf bytes.Buffer
	if e := genTF(&buf, pr, fts, cfg); e != nil {
		return e
	}

	cmd := tensorflowCmd(cwd)
	cmd.Stdin = &buf
	o, e := cmd.CombinedOutput()
	if e != nil || !strings.Contains(string(o), "Done predicting") {
		return fmt.Errorf("Prediction failed %v: \n%s", e, o)
	}

	return nil
}
