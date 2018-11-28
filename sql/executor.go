package sql

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/wangkuiyi/sqlfs"
	tar "github.com/wangkuiyi/tar"
	yaml "gopkg.in/yaml.v2"
)

const (
	workDir = `/tmp`
)

func run(slctStmt string, cfg *mysql.Config) error {
	sqlParse(newLexer(slctStmt))

	fts, err := verify(&parseResult, cfg)

	if parseResult.train {
		err = train(&parseResult, fts, cfg)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("Inference not implemented.\n")
	}
	return nil
}

func train(pr *extendedSelect, fts fieldTypes, cfg *mysql.Config) error {
	var program bytes.Buffer
	err := generateTFProgram(&program, pr, fts, cfg)
	if err != nil {
		return err
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

	err = saveModel(pr.save, cfg)
	if err != nil {
		return err
	}

	return nil
}

func getModelFilePrefix(modelDir string) (prefix string, e error) {
	f, e := os.Open(filepath.Join(modelDir, `checkpoint`))
	if e != nil {
		return "", e
	}
	defer func() { e = f.Close() }()

	m := map[string]string{}
	e = yaml.NewDecoder(f).Decode(m)
	if e != nil {
		return "", fmt.Errorf("Yaml Unmarshal: %v", e)
	}
	return m["model_checkpoint_path"], nil
}

func saveModel(modelName string, cfg *mysql.Config) (e error) {
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		return e
	}
	defer db.Close()

	dir := filepath.Join(workDir, modelName)
	prefix, e := getModelFilePrefix(dir)
	if e != nil {
		return e
	}

	sqlfn := fmt.Sprintf("sqlflow_models.%s", modelName)
	sqlf, e := sqlfs.Create(db, sqlfn)
	if e != nil {
		return fmt.Errorf("Cannot create sqlfs file %s: %v", sqlfn, e)
	}
	defer func() { e = sqlf.Close() }()

	inc := func(dir string, fi os.FileInfo) bool { return strings.HasPrefix(fi.Name(), prefix) }
	return tar.Tar(sqlf, dir, inc, true)
}
