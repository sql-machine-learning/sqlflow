// Copyright 2020 The SQLFlow Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sql

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"sqlflow.org/goalisa"
	"sqlflow.org/gomaxcompute"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/ir"
	"sqlflow.org/sqlflow/pkg/sql/codegen/pai"
)

var tarball = "job.tar.gz"

type paiSubmitter struct{ *defaultSubmitter }

func randStringRunes(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const lettersAndDigits = letters + "0123456789"
	b := make([]byte, n)
	// do not start from digit
	b[0] = letters[rand.Intn(len(letters))]
	for i := 1; i < len(b); i++ {
		b[i] = lettersAndDigits[rand.Intn(len(lettersAndDigits))]
	}
	return string(b)
}

func createTmpTableFromSelect(selectStmt, dataSource string) (string, string, error) {
	db, err := database.OpenAndConnectDB(dataSource)
	defer db.Close()
	if err != nil {
		return "", "", err
	}
	tableName := randStringRunes(16)
	// FIXME(typhoonzero): only work if specify database name in connect string.
	databaseName, err := getDatabaseNameFromDSN(dataSource)
	// NOTE(typhoonzero): MaxCompute do not support "CREATE	TABLE XXX AS (SELECT ...)"
	createSQL := fmt.Sprintf("CREATE TABLE %s AS %s", tableName, selectStmt)
	log.Printf(createSQL)
	_, err = db.Exec(createSQL)
	return databaseName, tableName, err
}

func dropTmpTables(tableNames []string, dataSource string) error {
	db, err := database.OpenAndConnectDB(dataSource)
	defer db.Close()
	if err != nil {
		return err
	}
	for _, tbName := range tableNames {
		log.Printf("drop tmp table %s", tbName)
		if tbName != "" {
			_, err = db.Exec(fmt.Sprintf("DROP TABLE %s", tbName))
			if err != nil {
				return err
			}
		}
	}
	return nil
}
func getDatabaseNameFromDSN(dataSource string) (string, error) {
	driverName, datasourceName, err := database.ParseURL(dataSource)
	if err != nil {
		return "", err
	}
	if driverName == "maxcompute" {
		cfg, err := gomaxcompute.ParseDSN(datasourceName)
		if err != nil {
			return "", err
		}
		return cfg.Project, nil
	} else if driverName == "alisa" {
		cfg, err := goalisa.ParseDSN(datasourceName)
		if err != nil {
			return "", err
		}
		return cfg.Project, nil
	}
	return "", fmt.Errorf("driver should be in ['maxcompute', 'alisa']")
}

func createTempTrainAndValTable(trainSelect, validSelect, datasource string) (string, string, error) {
	// TODO(typhoonzero): Do **NOT** create tmp table when the select statement is like:
	// "SELECT fields,... FROM table"
	dbName, tableName, err := createTmpTableFromSelect(trainSelect, datasource)
	if err != nil {
		return "", "", err
	}
	tmpTrainTable := strings.Join([]string{dbName, tableName}, ".")
	tmpValTable := ""
	if validSelect != "" {
		dbName, tableName, err := createTmpTableFromSelect(validSelect, datasource)
		if err != nil {
			return "", "", err
		}
		tmpValTable = strings.Join([]string{dbName, tableName}, ".")
	}
	return tmpTrainTable, tmpValTable, nil
}

// Possible situations:
//
// 1. argo mode server: generate a step running: repl -e "repl -e \"select * from xx to train\""
// 2. non-argo mode server | repl -e: create tmp table in go, and use it to train
func (s *paiSubmitter) ExecuteTrain(cl *ir.TrainStmt) (e error) {
	cl.TmpTrainTable, cl.TmpValidateTable, e = createTempTrainAndValTable(cl.Select, cl.ValidationSelect, s.Session.DbConnStr)
	if e != nil {
		return
	}
	defer dropTmpTables([]string{cl.TmpTrainTable, cl.TmpValidateTable}, s.Session.DbConnStr)

	ossModelPath, e := getModelPath(cl.Into, s.Session)
	if e != nil {
		return e
	}
	scriptPath := fmt.Sprintf("file://%s/%s", s.Cwd, tarball)
	code, paiCmd, requirements, e := pai.Train(cl, s.Session, scriptPath, cl.Into, ossModelPath, s.Cwd)
	if e != nil {
		return e
	}
	return s.submitPAITask(code, paiCmd, requirements)
}

func (s *paiSubmitter) submitPAITask(code, paiCmd, requirements string) error {
	if e := achieveResource(s.Cwd, code, requirements, tarball); e != nil {
		return e
	}
	_, datasourceName, e := database.ParseURL(s.Session.DbConnStr)
	if e != nil {
		return e
	}
	cfg, e := gomaxcompute.ParseDSN(datasourceName)
	if e != nil {
		return e
	}
	cmd := exec.Command("odpscmd", "-u", cfg.AccessID, "-p", cfg.AccessKey, "--project", cfg.Project, "--endpoint", cfg.Endpoint, "-e", paiCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed %s, %s, %v", cmd, out, err)
	}
	return nil
}

func (s *paiSubmitter) ExecutePredict(cl *ir.PredictStmt) error {
	// TODO(typhoonzero): Do **NOT** create tmp table when the select statement is like:
	// "SELECT fields,... FROM table"
	dbName, tableName, err := createTmpTableFromSelect(cl.Select, s.Session.DbConnStr)
	if err != nil {
		return err
	}
	cl.TmpPredictTable = strings.Join([]string{dbName, tableName}, ".")
	defer dropTmpTables([]string{cl.TmpPredictTable}, s.Session.DbConnStr)

	// format resultTable name to "db.table" to let the codegen form a submitting
	// argument of format "odps://project/tables/table_name"
	resultTableParts := strings.Split(cl.ResultTable, ".")
	if len(resultTableParts) == 1 {
		dbName, err := getDatabaseNameFromDSN(s.Session.DbConnStr)
		if err != nil {
			return err
		}
		cl.ResultTable = fmt.Sprintf("%s.%s", dbName, cl.ResultTable)
	}
	if e := createPredictionTableFromIR(cl, s.Db, s.Session); e != nil {
		return e
	}

	ossModelPath, e := getModelPath(cl.Using, s.Session)
	if e != nil {
		return e
	}
	modelType, _, err := getOSSSavedModelType(ossModelPath)
	if err != nil {
		return err
	}
	scriptPath := fmt.Sprintf("file://%s/%s", s.Cwd, tarball)
	code, paiCmd, requirements, e := pai.Predict(cl, s.Session, scriptPath, cl.Using, ossModelPath, s.Cwd, modelType)
	if e != nil {
		return e
	}
	return s.submitPAITask(code, paiCmd, requirements)
}

func (s *paiSubmitter) ExecuteExplain(cl *ir.ExplainStmt) error {
	// TODO(typhoonzero): Do **NOT** create tmp table when the select statement is like:
	// "SELECT fields,... FROM table"
	dbName, tableName, err := createTmpTableFromSelect(cl.Select, s.Session.DbConnStr)
	if err != nil {
		return err
	}
	cl.TmpExplainTable = strings.Join([]string{dbName, tableName}, ".")
	defer dropTmpTables([]string{cl.TmpExplainTable}, s.Session.DbConnStr)

	ossModelPath, e := getModelPath(cl.ModelName, s.Session)
	if e != nil {
		return e
	}

	modelType, estimator, err := getOSSSavedModelType(ossModelPath)
	if err != nil {
		return err
	}
	// format resultTable name to "db.table" to let the codegen form a submitting
	// argument of format "odps://project/tables/table_name"
	// ModelTypePAIML do not need to create explain result manually, PAI will
	// create the result table.
	if cl.Into != "" && modelType != pai.ModelTypePAIML {
		resultTableParts := strings.Split(cl.Into, ".")
		if len(resultTableParts) == 1 {
			dbName, err := getDatabaseNameFromDSN(s.Session.DbConnStr)
			if err != nil {
				return err
			}
			cl.Into = fmt.Sprintf("%s.%s", dbName, cl.Into)
		}
		db, err := database.OpenAndConnectDB(s.Session.DbConnStr)
		if err != nil {
			return err
		}
		err = createExplainResultTable(db, cl, cl.Into, modelType, estimator)
		if err != nil {
			return err
		}
	}
	scriptPath := fmt.Sprintf("file://%s/%s", s.Cwd, tarball)
	code, paiCmd, requirements, e := pai.Explain(cl, s.Session, scriptPath, cl.ModelName, ossModelPath, s.Cwd, modelType)
	if e != nil {
		return e
	}
	return s.submitPAITask(code, paiCmd, requirements)
}

// getOSSSavedModelType returns the saved model type when training, can be:
// 1. randomforests: model is saved by pai
// 2. xgboost: on OSS with model file xgboost_model_desx
// 3. PAI tensorflow models: on OSS with meta file: tensorflow_model_desc
func getOSSSavedModelType(modelName string) (modelType int, estimator string, err error) {
	// FIXME(typhoonzero): if the model not exist on OSS, assume it's a random forest model
	// should use a general method to fetch the model and see the model type.
	endpoint := os.Getenv("SQLFLOW_OSS_MODEL_ENDPOINT")
	ak := os.Getenv("SQLFLOW_OSS_AK")
	sk := os.Getenv("SQLFLOW_OSS_SK")
	if endpoint == "" || ak == "" || sk == "" {
		err = fmt.Errorf("must define SQLFLOW_OSS_MODEL_ENDPOINT, SQLFLOW_OSS_AK, SQLFLOW_OSS_SK when using submitter maxcompute")
		return
	}
	// NOTE(typhoonzero): PAI Tensorflow need SQLFLOW_OSS_CHECKPOINT_DIR, get bucket name from it
	ossCheckpointDir := os.Getenv("SQLFLOW_OSS_CHECKPOINT_DIR")
	ckptParts := strings.Split(ossCheckpointDir, "?")
	if len(ckptParts) != 2 {
		err = fmt.Errorf("SQLFLOW_OSS_CHECKPOINT_DIR got wrong format")
		return
	}
	urlParts := strings.Split(ckptParts[0], "://")
	if len(urlParts) != 2 {
		err = fmt.Errorf("SQLFLOW_OSS_CHECKPOINT_DIR got wrong format")
	}
	bucketName := strings.Split(urlParts[1], "/")[0]

	cli, err := oss.New(endpoint, ak, sk)
	if err != nil {
		return
	}
	bucket, err := cli.Bucket(bucketName)
	if err != nil {
		return
	}
	ret, err := bucket.IsObjectExist(modelName + "/tensorflow_model_desc")
	if err != nil {
		return
	}
	if ret {
		modelType = pai.ModelTypeTF
		var buf []byte
		err = bucket.GetObjectToFile(modelName+"/tensorflow_model_desc_estimator", "tmp_estimator_name")
		if err != nil {
			return
		}
		buf, err = ioutil.ReadFile("tmp_estimator_name")
		estimator = string(buf)
		return
	}
	ret, err = bucket.IsObjectExist(modelName + "/xgboost_model_desc")
	if err != nil {
		return
	}
	if ret {
		modelType = pai.ModelTypeXGBoost
		return
	}
	modelType = pai.ModelTypePAIML
	return
}

func getCreateShapResultSQL(db *database.DB, tableName string, selectStmt string, labelCol string) (string, error) {
	// create table to record shap values for every feature for each sample.
	flds, _, err := getColumnTypes(selectStmt, db)
	if err != nil {
		return "", err
	}
	columnDefList := []string{}
	for _, fieldName := range flds {
		if fieldName != labelCol {
			columnDefList = append(columnDefList, fmt.Sprintf("%s STRING", fieldName))
		}
	}
	createStmt := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (%s);`, tableName, strings.Join(columnDefList, ","))
	return createStmt, nil
}

func createExplainResultTable(db *database.DB, ir *ir.ExplainStmt, tableName string, modelType int, estimator string) error {
	dropStmt := fmt.Sprintf(`DROP TABLE IF EXISTS %s;`, tableName)
	var e error
	if _, e = db.Exec(dropStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", dropStmt, e)
	}
	createStmt := ""
	if modelType == pai.ModelTypeTF {
		if strings.HasPrefix(estimator, "BoostedTrees") {
			columnDef := ""
			if db.DriverName == "mysql" {
				columnDef = "(feature VARCHAR(255), dfc FLOAT, gain FLOAT)"
			} else {
				// Hive & MaxCompute
				columnDef = "(feature STRING, dfc STRING, gain STRING)"
			}
			createStmt = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s %s;`, tableName, columnDef)
		} else {
			labelCol, ok := ir.Attributes["label_col"]
			if !ok {
				return fmt.Errorf("need to specify WITH label_col=lable_col_name when explaining deep models")
			}
			createStmt, e = getCreateShapResultSQL(db, tableName, ir.Select, labelCol.(string))
			if e != nil {
				return e
			}
		}
	} else if modelType == pai.ModelTypeXGBoost {
		labelCol, ok := ir.Attributes["label_col"]
		if !ok {
			return fmt.Errorf("need to specify WITH label_col=lable_col_name when explaining xgboost models")
		}
		createStmt, e = getCreateShapResultSQL(db, tableName, ir.Select, labelCol.(string))
		if e != nil {
			return e
		}
	} else {
		return fmt.Errorf("not supported modelType %d for creating Explain result table", modelType)
	}

	if _, e := db.Exec(createStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", createStmt, e)
	}
	return nil
}

func achieveResource(cwd, entryCode, requirements, tarball string) error {
	if err := writeFile(filepath.Join(cwd, entryFile), entryCode); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(cwd, "requirements.txt"), requirements); err != nil {
		return err
	}

	path, err := findPyModulePath("sqlflow_submitter")
	if err != nil {
		return err
	}
	cmd := exec.Command("cp", "-r", path, ".")
	cmd.Dir = cwd
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed %s, %v", cmd, err)
	}

	cmd = exec.Command("tar", "czf", tarball, "./sqlflow_submitter", entryFile, "requirements.txt")
	cmd.Dir = cwd
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed %s, %v", cmd, err)
	}
	return nil
}

func (s *paiSubmitter) GetTrainStmtFromModel() bool { return false }
