package sql

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/wangkuiyi/sqlfs"
)

const (
	workDir = `/tmp`
)

func run(slct string, cfg *mysql.Config) error {
	sqlParse(newLexer(slct))
	fts, e := verify(&parseResult, cfg)
	if e != nil {
		return e
	}

	if parseResult.train {
		if e := train(&parseResult, fts, cfg); e != nil {
			return e
		}
	} else {
		return fmt.Errorf("inference not implemented")
	}

	return nil
}

func train(pr *extendedSelect, fts fieldTypes, cfg *mysql.Config) error {
	var program bytes.Buffer
	if e := generateTFProgram(&program, pr, fts, cfg); e != nil {
		return e
	}

	cmd := tensorflowCmd()
	cmd.Stdin = bytes.NewReader(program.Bytes())
	o, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	if !strings.Contains(string(o), "Done training") {
		return fmt.Errorf(string(o) + "\nTraining failed")
	}

	if err = saveTemplateFiller(pr, fts, cfg); err != nil {
		return err
	}

	return saveModelToDB(pr.save, cfg)
}

func saveTemplateFiller(pr *extendedSelect, fts fieldTypes, cfg *mysql.Config) error {
	r, err := generateTemplate(pr, fts, cfg)
	if err != nil {
		return err
	}
	fn := filepath.Join(workDir, r.Save, "trainTemplateFiller.gob")
	file, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	return encoder.Encode(r)
}

func saveModelToDB(modelName string, cfg *mysql.Config) (e error) {
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		return e
	}
	defer db.Close()

	sqlfn := fmt.Sprintf("sqlflow_models.%s", modelName)
	sqlf, e := sqlfs.Create(db, sqlfn)
	if e != nil {
		return fmt.Errorf("Cannot create sqlfs file %s: %v", sqlfn, e)
	}
	defer func() { e = sqlf.Close() }()

	dir := filepath.Join(workDir, modelName)
	cmd := exec.Command("tar", "Pczf", "-", dir)
	cmd.Stdout = sqlf

	return cmd.Run()
}
