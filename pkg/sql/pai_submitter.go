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
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"sqlflow.org/gomaxcompute"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/ir"
	"sqlflow.org/sqlflow/pkg/sql/codegen/pai"
)

const (
	tarball    = "job.tar.gz"
	paramsFile = "params.txt"

	// lifecycleOnTmpTable indicates 7 days for the temporary table
	// which create from SELECT statement
	lifecycleOnTmpTable = 7
)

var reODPSLogURL = regexp.MustCompile(`http://logview.*`)

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
	if err != nil {
		return "", "", err
	}
	defer db.Close()
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
	if err != nil {
		return err
	}
	defer db.Close()
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
	if ossAk == "" || ossSk == "" || ossEp == "" {
		return fmt.Errorf("must define SQLFLOW_OSS_AK, SQLFLOW_OSS_SK, SQLFLOW_OSS_MODEL_ENDPOINT when submitting to PAI")
	}

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
	return nil
}

// Possible situations:
//
// 1. argo mode server: generate a step running: bash -c "repl -e \"select * from xx to train\""
// 2. non-argo mode server | repl -e: create tmp table in go, and use it to train
func (s *paiSubmitter) ExecuteTrain(cl *ir.TrainStmt) (e error) {
	cl.TmpTrainTable, cl.TmpValidateTable, e = createTempTrainAndValTable(cl.Select, cl.ValidationSelect, s.Session.DbConnStr)
	if e != nil {
		return
	}
	defer dropTmpTables([]string{cl.TmpTrainTable, cl.TmpValidateTable}, s.Session.DbConnStr)

	ossModelPathToSave, e := getModelPath(cl.Into, s.Session)
	if e != nil {
		return e
	}

	ossModelPathToLoad := ""
	if cl.PreTrainedModel != "" {
		ossModelPathToLoad, e = getModelPath(cl.PreTrainedModel, s.Session)
		if e != nil {
			return e
		}
	}

	// NOTE(sneaxiy): should be careful whether there would be file conflict
	// if we do not remove the original OSS files.
	if ossModelPathToLoad == "" || ossModelPathToSave != ossModelPathToLoad {
		currProject, e := database.GetDatabaseName(s.Session.DbConnStr)
		if e != nil {
			return e
		}
		e = cleanOSSModelPath(ossModelPathToSave+"/", currProject)
		if e != nil {
			return e
		}
	}

	scriptPath := fmt.Sprintf("file://%s/%s", s.Cwd, tarball)
	paramsPath := fmt.Sprintf("file://%s/%s", s.Cwd, paramsFile)
	if err := createPAIHyperParamFile(s.Cwd, paramsFile, ossModelPathToSave); err != nil {
		return err
	}
	code, paiCmd, requirements, e := pai.Train(cl, s.Session, scriptPath, paramsPath, cl.Into, ossModelPathToSave,
		ossModelPathToLoad, s.Cwd)
	if e != nil {
		return e
	}
	return s.submitPAITask(code, paiCmd, requirements, cl.Estimator)
}

func cleanOSSModelPath(ossModelPath, project string) error {
	bucket, err := getOSSModelBucket(project)
	if err != nil {
		return err
	}
	return deleteDirRecursive(bucket, ossModelPath)
}

func (s *paiSubmitter) submitPAITask(code, paiCmd, requirements, estimator string) error {
	if e := achieveResource(s.Cwd, code, requirements, tarball, estimator); e != nil {
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
	cw := &logChanWriter{wr: s.Writer}
	var output bytes.Buffer
	w := io.MultiWriter(cw, &output)
	defer cw.Close()
	cmd := exec.Command("odpscmd", "--instance-priority", "9", "-u", cfg.AccessID, "-p", cfg.AccessKey, "--project", cfg.Project, "--endpoint", cfg.Endpoint, "-e", paiCmd)
	cmd.Stdout, cmd.Stderr = w, w
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("failed: %v\n%sProgram%[2]s\n%s\n%[2]sOutput%[2]s\n%[4]v", e, "==========", paiCmd, output.String())
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
	modelType, estimator, err := getOSSSavedModelType(ossModelPath, currProject)
	if err != nil {
		return err
	}
	scriptPath := fmt.Sprintf("file://%s/%s", s.Cwd, tarball)
	paramsPath := fmt.Sprintf("file://%s/%s", s.Cwd, paramsFile)
	if err := createPAIHyperParamFile(s.Cwd, paramsFile, ossModelPath); err != nil {
		return err
	}
	code, paiCmd, requirements, e := pai.Predict(cl, s.Session, scriptPath, paramsPath, cl.Using, ossModelPath, s.Cwd, modelType)
	if e != nil {
		return e
	}
	return s.submitPAITask(code, paiCmd, requirements, estimator)
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
		defer db.Close()
		err = createExplainResultTable(db, cl, cl.Into, modelType, estimator)
		if err != nil {
			return err
		}
	}
	scriptPath := fmt.Sprintf("file://%s/%s", s.Cwd, tarball)
	paramsPath := fmt.Sprintf("file://%s/%s", s.Cwd, paramsFile)
	if err := createPAIHyperParamFile(s.Cwd, paramsFile, ossModelPath); err != nil {
		return err
	}
	expn, e := pai.Explain(cl, s.Session, scriptPath, paramsPath, cl.ModelName, ossModelPath, s.Cwd, modelType)
	if e != nil {
		return e
	}

	if e = s.submitPAITask(expn.Code, expn.PaiCmd, expn.Requirements, estimator); e != nil {
		return e
	}
	if img, e := expn.Draw(); e == nil {
		s.Writer.Write(Figures{img, ""})
	}
	return e
}

func (s *paiSubmitter) ExecuteEvaluate(cl *ir.EvaluateStmt) error {
	// TODO(typhoonzero): Do **NOT** create tmp table when the select statement is like:
	// "SELECT fields,... FROM table"
	dbName, tableName, err := createTmpTableFromSelect(cl.Select, s.Session.DbConnStr)
	if err != nil {
		return err
	}
	cl.TmpEvaluateTable = strings.Join([]string{dbName, tableName}, ".")
	defer dropTmpTables([]string{cl.TmpEvaluateTable}, s.Session.DbConnStr)

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
		defer db.Close()
		// default always output evaluation loss
		metricNames := []string{"loss"}
		metricsAttr, ok := cl.Attributes["validation.metrics"]
		if ok {
			metricsList := strings.Split(metricsAttr.(string), ",")
			metricNames = append(metricNames, metricsList...)
		}
		err = createEvaluationResultTable(db, cl.Into, metricNames)
		if err != nil {
			return err
		}
	}
	scriptPath := fmt.Sprintf("file://%s/%s", s.Cwd, tarball)
	paramsPath := fmt.Sprintf("file://%s/%s", s.Cwd, paramsFile)
	if err := createPAIHyperParamFile(s.Cwd, paramsFile, ossModelPath); err != nil {
		return err
	}
	code, paiCmd, requirements, e := pai.Evaluate(cl, s.Session, scriptPath, paramsPath, cl.ModelName, ossModelPath, s.Cwd, modelType)
	if e != nil {
		return e
	}

	if e = s.submitPAITask(code, paiCmd, requirements, estimator); e != nil {
		return e
	}
	return e
}

// TODO(sneaxiy): need to add some tests to this function, but it requires
// optflow installed in docker image
func (s *paiSubmitter) ExecuteOptimize(cl *ir.OptimizeStmt) error {
	dbName, tableName, err := createTmpTableFromSelect(cl.Select, s.Session.DbConnStr)
	if err != nil {
		return err
	}
	defer dropTmpTables([]string{tableName}, s.Session.DbConnStr)

	db, err := database.OpenAndConnectDB(s.Session.DbConnStr)
	if err != nil {
		return err
	}
	defer db.Close()

	splittedResultTable := strings.SplitN(cl.ResultTable, ".", 2)
	var resultTable string
	if len(splittedResultTable) == 2 {
		if splittedResultTable[0] != dbName {
			return fmt.Errorf("database name of result table must be the same as source table")
		}
		resultTable = cl.ResultTable
	} else {
		resultTable = fmt.Sprintf("%s.%s", dbName, cl.ResultTable)
	}

	_, err = db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", resultTable))
	if err != nil {
		return err
	}

	err = generateOptFlowOptimizeCodeAndExecute(cl, s.defaultSubmitter, s.Session, s.Cwd, dbName, tableName, true)
	return err
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

func copyPythonPackage(packageName, dst string) error {
	path, e := findPyModulePath(packageName)
	if e != nil {
		return fmt.Errorf("Can not find Python package: %s", packageName)
	}
	cmd := exec.Command("cp", "-r", path, ".")
	cmd.Dir = dst
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed %s, %v", cmd, err)
	}
	return nil
}

func copyCustomPackage(estimator, dst string) error {
	modelNameParts := strings.Split(estimator, ".")
	pkgName := modelNameParts[0]
	if len(modelNameParts) == 2 && pkgName != "sqlflow_models" && pkgName != "xgboost" {
		return copyPythonPackage(pkgName, dst)
	}
	return nil
}

func achieveResource(cwd, entryCode, requirements, tarball, estimator string) error {
	if err := writeFile(filepath.Join(cwd, entryFile), entryCode); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(cwd, "requirements.txt"), requirements); err != nil {
		return err
	}
	// sqlflow_submitter and sqlflow_models are built-in packages.
	if err := copyPythonPackage("sqlflow_submitter", cwd); err != nil {
		return err
	}
	if err := copyPythonPackage("sqlflow_models", cwd); err != nil {
		return err
	}

	// add custom package if needed
	if err := copyCustomPackage(estimator, cwd); err != nil {
		return err
	}

	cmd := exec.Command("tar", "czf", tarball, "./sqlflow_submitter", "./sqlflow_models", entryFile, "requirements.txt")
	cmd.Dir = cwd
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed %s, %v", cmd, err)
	}
	return nil
}

func (s *paiSubmitter) GetTrainStmtFromModel() bool { return false }

func pickPAILogViewerURL(output string) []string {
	return reODPSLogURL.FindAllString(output, -1)
}
