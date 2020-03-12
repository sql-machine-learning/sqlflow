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
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"sqlflow.org/gomaxcompute"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/ir"
	"sqlflow.org/sqlflow/pkg/sql/codegen/pai"
)

const tarball = "job.tar.gz"
const paramsFile = "params.txt"

// lifecycleOnTmpTable indicates 7 days for the temporary table
// which create from SELECT statement
const lifecycleOnTmpTable = 7

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
	databaseName, err := database.GetDatabaseName(dataSource)
	if err != nil {
		return "", "", err
	}
	// NOTE(typhoonzero): MaxCompute do not support "CREATE	TABLE XXX AS (SELECT ...)"
	createSQL := fmt.Sprintf("CREATE TABLE %s LIFECYCLE %d AS %s", tableName, lifecycleOnTmpTable, selectStmt)
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
		// TODO(yancey1989): write log into pipe to avoid the wrong row
		// log.Printf("drop tmp table %s", tbName)
		if tbName != "" {
			_, err = db.Exec(fmt.Sprintf("DROP TABLE %s", tbName))
			if err != nil {
				return err
			}
		}
	}
	return nil
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

func createPAIHyperParamFile(cwd string, filename string, modelPath string) error {
	f, err := os.Create(fmt.Sprintf(path.Join(cwd, filename)))
	if err != nil {
		return err
	}
	defer f.Close()
	ossAk := os.Getenv("SQLFLOW_OSS_AK")
	ossSk := os.Getenv("SQLFLOW_OSS_SK")
	ossEp := os.Getenv("SQLFLOW_OSS_MODEL_ENDPOINT")
	hdfsCkpt := os.Getenv("SQLFLOW_HDFS_MODEL_CKPT_DIR")
	if ossAk == "" || ossSk == "" || ossEp == "" || hdfsCkpt == "" {
		return fmt.Errorf("must define SQLFLOW_OSS_AK, SQLFLOW_OSS_SK, SQLFLOW_OSS_MODEL_ENDPOINT, SQLFLOW_HDFS_MODEL_CKPT_DIR when submitting to PAI")
	}

	hdfsDir := fmt.Sprintf("%s/%s",
		strings.TrimRight(hdfsCkpt, "/"),
		strings.TrimLeft(modelPath, "/"))

	if _, err := f.Write([]byte(fmt.Sprintf("sqlflow_oss_ak=\"%s\"\n", ossAk))); err != nil {
		return err
	}
	if _, err := f.Write([]byte(fmt.Sprintf("sqlflow_oss_sk=\"%s\"\n", ossSk))); err != nil {
		return err
	}
	if _, err := f.Write([]byte(fmt.Sprintf("sqlflow_oss_ep=\"%s\"\n", ossEp))); err != nil {
		return err
	}
	ossModelURL := pai.OSSModelURL(modelPath)
	if _, err := f.Write([]byte(fmt.Sprintf("sqlflow_oss_modeldir=\"%s\"\n", ossModelURL))); err != nil {
		return err
	}
	if _, err := f.Write([]byte(fmt.Sprintf("sqlflow_hdfs_ckpt=\"%s\"\n", hdfsDir))); err != nil {
		return err
	}
	return nil
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

	currProject, e := database.GetDatabaseName(s.Session.DbConnStr)
	if e != nil {
		return e
	}
	e = cleanOSSModelPath(ossModelPath+"/", currProject)
	if e != nil {
		return e
	}
	scriptPath := fmt.Sprintf("file://%s/%s", s.Cwd, tarball)
	paramsPath := fmt.Sprintf("file://%s/%s", s.Cwd, paramsFile)
	if err := createPAIHyperParamFile(s.Cwd, paramsPath, ossModelPath); err != nil {
		return err
	}
	code, paiCmd, requirements, e := pai.Train(cl, s.Session, scriptPath, paramsPath, cl.Into, ossModelPath, s.Cwd)
	if e != nil {
		return e
	}
	return s.submitPAITask(code, paiCmd, requirements)
}

func cleanOSSModelPath(ossModelPath, project string) error {
	bucket, err := getOSSModelBucket(project)
	if err != nil {
		return err
	}
	return deleteDirRecursive(bucket, ossModelPath)
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
	cmd := exec.Command("odpscmd", "--instance-priority", "9", "-u", cfg.AccessID, "-p", cfg.AccessKey, "--project", cfg.Project, "--endpoint", cfg.Endpoint, "-e", paiCmd)
	if out, err := cmd.CombinedOutput(); err != nil {
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

	currProject, err := database.GetDatabaseName(s.Session.DbConnStr)
	if err != nil {
		return err
	}
	// format resultTable name to "db.table" to let the codegen form a submitting
	// argument of format "odps://project/tables/table_name"
	resultTableParts := strings.Split(cl.ResultTable, ".")
	if len(resultTableParts) == 1 {
		cl.ResultTable = fmt.Sprintf("%s.%s", currProject, cl.ResultTable)
	}
	if e := createPredictionTableFromIR(cl, s.Db, s.Session); e != nil {
		return e
	}

	ossModelPath, e := getModelPath(cl.Using, s.Session)
	if e != nil {
		return e
	}
	modelType, _, err := getOSSSavedModelType(ossModelPath, currProject)
	if err != nil {
		return err
	}
	scriptPath := fmt.Sprintf("file://%s/%s", s.Cwd, tarball)
	paramsPath := fmt.Sprintf("file://%s/%s", s.Cwd, paramsFile)
	if err := createPAIHyperParamFile(s.Cwd, paramsPath, ossModelPath); err != nil {
		return err
	}
	code, paiCmd, requirements, e := pai.Predict(cl, s.Session, scriptPath, paramsPath, cl.Using, ossModelPath, s.Cwd, modelType)
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

	currProject, err := database.GetDatabaseName(s.Session.DbConnStr)
	if err != nil {
		return err
	}
	modelType, estimator, err := getOSSSavedModelType(ossModelPath, currProject)
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
			cl.Into = fmt.Sprintf("%s.%s", currProject, cl.Into)
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
	paramsPath := fmt.Sprintf("file://%s/%s", s.Cwd, paramsFile)
	if err := createPAIHyperParamFile(s.Cwd, paramsPath, ossModelPath); err != nil {
		return err
	}
	expn, e := pai.Explain(cl, s.Session, scriptPath, paramsPath, cl.ModelName, ossModelPath, s.Cwd, modelType)
	if e != nil {
		return e
	}
	if e = s.submitPAITask(expn.Code, expn.PaiCmd, expn.Requirements); e != nil {
		return e
	}
	if img, e := expn.Draw(); e == nil {
		s.Writer.Write(Figures{img, ""})
	}
	return e
}

// getOSSModelBucket construct a bucket object. Argument project is used to get OSS checkpoint dir
// from environment variable for current MaxCompute project.
// FIXME(typhoonzero): use the same model bucket name e.g. sqlflow-models
func getOSSModelBucket(project string) (*oss.Bucket, error) {
	endpoint := os.Getenv("SQLFLOW_OSS_MODEL_ENDPOINT")
	ak := os.Getenv("SQLFLOW_OSS_AK")
	sk := os.Getenv("SQLFLOW_OSS_SK")
	if endpoint == "" || ak == "" || sk == "" {
		return nil, fmt.Errorf("must define SQLFLOW_OSS_MODEL_ENDPOINT, SQLFLOW_OSS_AK, SQLFLOW_OSS_SK when using submitter maxcompute")
	}

	cli, err := oss.New(endpoint, ak, sk)
	if err != nil {
		return nil, err
	}
	return cli.Bucket(pai.BucketName)
}

// getOSSSavedModelType returns the saved model type when training, can be:
// 1. randomforests: model is saved by pai
// 2. xgboost: on OSS with model file xgboost_model_desc
// 3. PAI tensorflow models: on OSS with meta file: tensorflow_model_desc
func getOSSSavedModelType(modelName string, project string) (modelType int, estimator string, err error) {
	// FIXME(typhoonzero): if the model not exist on OSS, assume it's a random forest model
	// should use a general method to fetch the model and see the model type.
	bucket, err := getOSSModelBucket(project)
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

// deleteDirRecursive recursively delete a directory on the OSS
func deleteDirRecursive(bucket *oss.Bucket, dir string) error {
	exists, err := bucket.IsObjectExist(dir)
	if err != nil {
		return err
	}
	if !exists {
		// if directory not exist, just go on.
		return nil
	}
	if !strings.HasSuffix(dir, "/") {
		return fmt.Errorf("dir to delete must end with /")
	}
	objectPathList := []string{}
	lor, err := bucket.ListObjects(oss.Prefix(dir), oss.Delimiter("/"))
	if err != nil {
		return err
	}
	for _, object := range lor.Objects {
		objectPathList = append(objectPathList, object.Key)
	}
	// delete sub dir first
	if len(lor.CommonPrefixes) > 0 {
		for _, subPrefix := range lor.CommonPrefixes {
			err := deleteDirRecursive(bucket, subPrefix)
			if err != nil {
				return err
			}
		}
	}
	_, err = bucket.DeleteObjects(objectPathList)
	if err != nil {
		return err
	}
	return nil
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

	// add sqlflow_submitter
	path, err := findPyModulePath("sqlflow_submitter")
	if err != nil {
		return err
	}
	cmd := exec.Command("cp", "-r", path, ".")
	cmd.Dir = cwd
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed %s, %v", cmd, err)
	}

	// add sqlflow_models
	path, err = findPyModulePath("sqlflow_models")
	if err != nil {
		return err
	}
	cmd = exec.Command("cp", "-r", path, ".")
	cmd.Dir = cwd
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed %s, %v", cmd, err)
	}

	cmd = exec.Command("tar", "czf", tarball, "./sqlflow_submitter", "./sqlflow_models", entryFile, "requirements.txt")
	cmd.Dir = cwd
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed %s, %v", cmd, err)
	}
	return nil
}

func (s *paiSubmitter) GetTrainStmtFromModel() bool { return false }
