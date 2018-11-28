package sql

import (
	"bytes"
	"database/sql"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/wangkuiyi/sqlflow/sql/sqlfile"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
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

func listModelFileNames(modelDir string) ([]string, error) {
	dat, err := ioutil.ReadFile(filepath.Join(modelDir, `checkpoint`))
	if err != nil {
		return nil, err
	}

	m := map[string]string{}
	err = yaml.Unmarshal(dat, m)
	if err != nil {
		return nil, fmt.Errorf("Yaml Unmarshal: %v", err)
	}
	modelFilePrefix := m["model_checkpoint_path"]

	files, err := ioutil.ReadDir(modelDir)
	if err != nil {
		return nil, err
	}

	rval := []string{}
	for _, f := range files {
		if strings.HasPrefix(f.Name(), modelFilePrefix) {
			rval = append(rval, f.Name())
		}
	}

	return rval, nil
}

// A model file is of name model.ckpt-16000.data-00000-of-00002
// The "." and "-" are special characters in SQL
// So we rename the table name from model.ckpt-16000.data-00000-of-00002
// to data_00000_of_00002
// This filename rewrite rule is actually reversible.
func encodeFileNameToTableName(fileName string) string {
	return strings.Replace(strings.Split(fileName, ".")[2], "-", "_", -1)
}

func saveModel(modelName string, cfg *mysql.Config) error {
	modelDir := filepath.Join(workDir, modelName)
	modelFileNames, err := listModelFileNames(modelDir)

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return err
	}
	defer db.Close()

	// store model files, model.ckpt*
	for _, fileName := range modelFileNames {
		tableName := encodeFileNameToTableName(fileName)
		w, err := sqlfile.Create(db, modelName+"."+tableName)
		if err != nil {
			return err
		}
		defer w.Close()

		src, err := os.Open(filepath.Join(modelDir, fileName))
		if err != nil {
			return err
		}
		defer src.Close()

		_, err = io.Copy(w, src)
		if err != nil {
			return err
		}

		fmt.Println("Successfully store", tableName)
	}

	// TODO(tonyyang-svail): store train model template


	return nil
}
