package sql

import (
	"bytes"
	"database/sql"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/wangkuiyi/sqlflow/sql/sqlfile"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	workDir = `/tmp`
)

func executeTrain(pr *extendedSelect, fts fieldTypes, cfg *mysql.Config) error {
	var program bytes.Buffer
	err := generateTFProgram(&program, pr, fts, cfg)
	if err != nil { return err }

	cmd := exec.Command("docker", "run",
		"--rm", "--network=host", "-i",
		"-v", workDir + ":/work",
		"-w", "/work",
		"sqlflow/sqlflow", "python")
	cmd.Stdin = bytes.NewReader(program.Bytes())
	o, err := cmd.CombinedOutput()
	if err != nil { return err }
	if !strings.Contains(string(o), "Done training") {
		return fmt.Errorf(string(o) + "\n Does contain Done training")
	}

	modelName := pr.save
	modelDir := filepath.Join(workDir, modelName)
	dat, err := ioutil.ReadFile(filepath.Join(modelDir, `checkpoint`))
	if err != nil { return err }

	regex, _ := regexp.Compile(`model_checkpoint_path: \"([a-z]+.[a-z]+.\d+)\"`)
	modelFilePrefix := regex.FindStringSubmatch(string(dat))[1]
	files, err := ioutil.ReadDir(modelDir)
	if err != nil { return err }

	db, err := sql.Open("mysql", cfg.FormatDSN())
	defer func() { db.Close() }()
	if err != nil { return err }

	// store model files, model.ckpt*
	for _, f := range files {
		if strings.HasPrefix(f.Name(), modelFilePrefix) {
			// A model file is of name model.ckpt-16000.data-00000-of-00002
			// The "." and "-" are special characters in SQL
			// So we rename the table name from model.ckpt-16000.data-00000-of-00002
			// to data_00000_of_00002
			tn := strings.Replace(strings.Split(f.Name(), ".")[2], "-", "_", -1)
			w, err := sqlfile.Create(db, modelName+"."+tn)
			if err != nil { return err }

			dat, err = ioutil.ReadFile(filepath.Join(modelDir, f.Name()))
			if err != nil { return err }

			n, err := w.Write(dat)
			if n != len(dat) {
				return fmt.Errorf("Writing %s expect %d, got %d\n", f.Name(), len(dat), n)
			}
			if err != nil { return err }
		}
	}

	// TODO(tonyyang-svail): store train model template
	// trainTemplate, err := newTemplateFiller(pr, fts, cfg)
	// if err != nil { return err }
	// w, err := sqlfile.Create(db, modelName+"."+"trainTemplate")
	// enc := gob.NewEncoder(w)
	// enc.Encode(trainTemplate)
	// if err != nil { return err }

	return nil
}
