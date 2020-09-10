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

package executor

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"sqlflow.org/sqlflow/go/verifier"

	"sqlflow.org/sqlflow/go/codegen/optimize"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"sqlflow.org/gomaxcompute"
	"sqlflow.org/sqlflow/go/codegen/pai"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/ir"
	"sqlflow.org/sqlflow/go/model"
	pb "sqlflow.org/sqlflow/go/proto"
	"sqlflow.org/sqlflow/go/randstring"
)

const (
	tarball    = "job.tar.gz"
	paramsFile = "params.txt"
	// Let's guess the OSS auth error from message body:
	// ...
	// FAILED: Failed 26***306:kNotFound:The role_arn you provide not exists in OSS auth service. Please check carefully.
	// ...
	ossAuthErrorMsg = "The role_arn you provide not exists in OSS auth service"
	// lifecycleOnTmpTable indicates 7 days for the temporary table
	// which create from SELECT statement
	lifecycleOnTmpTable = 7
)

var reODPSLogURL = regexp.MustCompile(`http://logview.*`)

type paiExecutor struct{ *pythonExecutor }

func createTmpTableFromSelect(selectStmt, dataSource string) (string, string, error) {
	db, err := database.OpenAndConnectDB(dataSource)
	if err != nil {
		return "", "", err
	}
	defer db.Close()
	tableName := randstring.Generate(16)
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

func preExecuteTrainOnPAI(cl *ir.TrainStmt, session *pb.Session) (e error) {
	// create tmp table for training and validating
	cl.TmpTrainTable, cl.TmpValidateTable, e = createTempTrainAndValTable(cl.Select, cl.ValidationSelect, session.DbConnStr)
	if e != nil {
		return e
	}
	return nil
}

// getPaiTrainCode returns (code, paiCmd, requirements, error) for submit.
func getPaiTrainCode(s *pythonExecutor, trainStmt *ir.TrainStmt) (string, string, string, error) {
	if err := preExecuteTrainOnPAI(trainStmt, s.Session); err != nil {
		return "", "", "", err
	}

	ossModelPathToSave, e := getModelPath(trainStmt.Into, s.Session)
	if e != nil {
		return "", "", "", e
	}

	ossModelPathToLoad := ""
	if trainStmt.PreTrainedModel != "" {
		ossModelPathToLoad, e = getModelPath(trainStmt.PreTrainedModel, s.Session)
		if e != nil {
			return "", "", "", e
		}
	}

	currProject, e := database.GetDatabaseName(s.Session.DbConnStr)
	if e != nil {
		return "", "", "", e
	}
	// NOTE(sneaxiy): should be careful whether there would be file conflict
	// if we do not remove the original OSS files.
	if ossModelPathToLoad == "" || ossModelPathToSave != ossModelPathToLoad {
		e = cleanOSSModelPath(ossModelPathToSave+"/", currProject)
		if e != nil {
			return "", "", "", e
		}
	}

	scriptPath := fmt.Sprintf("file://%s/%s", s.Cwd, tarball)
	paramsPath := fmt.Sprintf("file://%s/%s", s.Cwd, paramsFile)
	if err := createPAIHyperParamFile(s.Cwd, paramsFile, ossModelPathToSave); err != nil {
		return "", "", "", err
	}
	code, paiCmd, requirements, e := pai.Train(trainStmt, s.Session, scriptPath, paramsPath, trainStmt.Into, ossModelPathToSave,
		ossModelPathToLoad, s.Cwd)
	if e != nil {
		return "", "", "", e
	}
	return code, paiCmd, requirements, nil
}

// Possible situations:
//
// 1. argo mode server: generate a step running: bash -c "repl -e \"select * from xx to train\""
// 2. non-argo mode server | repl -e: create tmp table in go, and use it to train
func (s *paiExecutor) ExecuteTrain(cl *ir.TrainStmt) (e error) {
	code, paiCmd, requirements, e := getPaiTrainCode(s.pythonExecutor, cl)
	// drop table should be run even if there's error.
	defer dropTmpTables([]string{cl.TmpTrainTable, cl.TmpValidateTable}, s.Session.DbConnStr)
	if e != nil {
		return e
	}

	if e = s.submitPAITask(code, paiCmd, requirements, cl.Estimator); e != nil {
		return e
	}

	ossModelPathToSave, e := getModelPath(cl.Into, s.Session)
	if e != nil {
		return e
	}
	currProject, e := database.GetDatabaseName(s.Session.DbConnStr)
	if e != nil {
		return e
	}
	// download model from OSS to local cwd and save to sqlfs
	// NOTE(typhoonzero): model in sqlfs will be used by sqlflow model zoo currently
	// should use the model in sqlfs when predicting.
	if e = downloadOSSModel(ossModelPathToSave+"/", currProject); e != nil {
		return e
	}
	m := model.New(s.Cwd, cl.OriginalSQL)
	return m.Save(cl.Into, s.Session)
}

func downloadOSSModel(ossModelPath, project string) error {
	bucket, err := getOSSModelBucket(project)
	if err != nil {
		return err
	}
	if !strings.HasSuffix(ossModelPath, "/") {
		return fmt.Errorf("dir to download must end with /")
	}
	localDirParts := strings.Split(ossModelPath, "/")
	localDir := localDirParts[len(localDirParts)-2] // the last char must be /
	return downloadDirRecursive(bucket, ossModelPath, localDir+"/")
}

// downloadDirRecursive recursively download a directory on the OSS
func downloadDirRecursive(bucket *oss.Bucket, dir, localDir string) error {
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return err
	}

	objectPathList := []string{}
	lor, err := bucket.ListObjects(oss.Prefix(dir), oss.Delimiter("/"))
	if err != nil {
		return err
	}
	for _, object := range lor.Objects {
		objectPathList = append(objectPathList, object.Key)
	}
	// download sub dir first
	if len(lor.CommonPrefixes) > 0 {
		for _, subPrefix := range lor.CommonPrefixes {
			subLocalDir := strings.TrimPrefix(subPrefix, dir)
			err := downloadDirRecursive(bucket, subPrefix, fmt.Sprintf("%s%s", localDir, subLocalDir))
			if err != nil {
				return err
			}
		}
	}
	for _, objPath := range objectPathList {
		if strings.HasSuffix(objPath, "/") {
			continue
		}
		localFullPath := ""
		if strings.HasPrefix(objPath, dir) {
			localObjPath := strings.TrimPrefix(objPath, dir)
			localFullPath = fmt.Sprintf("%s%s", localDir, localObjPath)
		} else {
			return fmt.Errorf("objpath: %s does not match model dir %s", objPath, dir)
		}
		if err := bucket.GetObjectToFile(objPath, localFullPath); err != nil {
			return err
		}
	}
	return nil
}

func cleanOSSModelPath(ossModelPath, project string) error {
	bucket, err := getOSSModelBucket(project)
	if err != nil {
		return err
	}
	return deleteDirRecursive(bucket, ossModelPath)
}

func (s *paiExecutor) submitPAITask(code, paiCmd, requirements, estimator string) error {
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

func getPaiPredictCode(s *pythonExecutor, cl *ir.PredictStmt) (string, string, string, string, error) {
	// TODO(typhoonzero): Do **NOT** create tmp table when the select statement is like:
	// "SELECT fields,... FROM table"
	dbName, tableName, err := createTmpTableFromSelect(cl.Select, s.Session.DbConnStr)
	if err != nil {
		return "", "", "", "", err
	}
	cl.TmpPredictTable = strings.Join([]string{dbName, tableName}, ".")

	currProject, err := database.GetDatabaseName(s.Session.DbConnStr)
	if err != nil {
		return "", "", "", "", err
	}
	// format resultTable name to "db.table" to let the codegen form a submitting
	// argument of format "odps://project/tables/table_name"
	resultTableParts := strings.Split(cl.ResultTable, ".")
	if len(resultTableParts) == 1 {
		cl.ResultTable = fmt.Sprintf("%s.%s", currProject, cl.ResultTable)
	}
	if e := createPredictionResultTable(cl, s.Db, s.Session); e != nil {
		return "", "", "", "", e
	}

	ossModelPath, e := getModelPath(cl.Using, s.Session)
	if e != nil {
		return "", "", "", "", e
	}
	modelType, estimator, err := getOSSSavedModelType(ossModelPath, currProject)
	if err != nil {
		return "", "", "", "", err
	}
	scriptPath := fmt.Sprintf("file://%s/%s", s.Cwd, tarball)
	paramsPath := fmt.Sprintf("file://%s/%s", s.Cwd, paramsFile)
	if err := createPAIHyperParamFile(s.Cwd, paramsFile, ossModelPath); err != nil {
		return "", "", "", "", err
	}
	code, paiCmd, requirements, err := pai.Predict(cl, s.Session, scriptPath, paramsPath, cl.Using, ossModelPath, s.Cwd, modelType)
	if err != nil {
		return "", "", "", "", err
	}
	return code, paiCmd, requirements, estimator, nil
}

func (s *paiExecutor) ExecutePredict(cl *ir.PredictStmt) error {
	code, paiCmd, requirements, estimator, err := getPaiPredictCode(s.pythonExecutor, cl)
	defer dropTmpTables([]string{cl.TmpPredictTable}, s.Session.DbConnStr)
	if err != nil {
		return err
	}
	return s.submitPAITask(code, paiCmd, requirements, estimator)
}

func getPaiExplainCode(s *pythonExecutor, cl *ir.ExplainStmt) (*pai.ExplainRender, string, error) {
	// TODO(typhoonzero): Do **NOT** create tmp table when the select statement is like:
	// "SELECT fields,... FROM table"
	dbName, tableName, err := createTmpTableFromSelect(cl.Select, s.Session.DbConnStr)
	if err != nil {
		return nil, "", err
	}
	cl.TmpExplainTable = strings.Join([]string{dbName, tableName}, ".")

	ossModelPath, e := getModelPath(cl.ModelName, s.Session)
	if e != nil {
		return nil, "", e
	}

	currProject, err := database.GetDatabaseName(s.Session.DbConnStr)
	if err != nil {
		return nil, "", err
	}
	modelType, estimator, err := getOSSSavedModelType(ossModelPath, currProject)
	if err != nil {
		return nil, "", err
	}
	// format resultTable name to "db.table" to let the codegen form a submitting
	// argument of format "odps://project/tables/table_name"
	// PAIML do not need to create explain result manually, PAI will
	// create the result table.
	if cl.Into != "" && modelType != model.PAIML {
		resultTableParts := strings.Split(cl.Into, ".")
		if len(resultTableParts) == 1 {
			cl.Into = fmt.Sprintf("%s.%s", currProject, cl.Into)
		}
		db, err := database.OpenAndConnectDB(s.Session.DbConnStr)
		if err != nil {
			return nil, "", err
		}
		defer db.Close()
		err = createExplainResultTable(db, cl, cl.Into, modelType, estimator)
		if err != nil {
			return nil, "", err
		}
	}
	scriptPath := fmt.Sprintf("file://%s/%s", s.Cwd, tarball)
	paramsPath := fmt.Sprintf("file://%s/%s", s.Cwd, paramsFile)
	if err := createPAIHyperParamFile(s.Cwd, paramsFile, ossModelPath); err != nil {
		return nil, "", err
	}
	expn, e := pai.Explain(cl, s.Session, scriptPath, paramsPath, cl.ModelName, ossModelPath, s.Cwd, modelType)
	if e != nil {
		return nil, "", e
	}
	return expn, estimator, nil
}

func (s *paiExecutor) ExecuteExplain(cl *ir.ExplainStmt) error {
	expn, estimator, e := getPaiExplainCode(s.pythonExecutor, cl)
	defer dropTmpTables([]string{cl.TmpExplainTable}, s.Session.DbConnStr)
	if e != nil {
		return e
	}

	if e := s.submitPAITask(expn.Code, expn.PaiCmd, expn.Requirements, estimator); e != nil {
		return e
	}
	if img, e := expn.Draw(); e == nil {
		s.Writer.Write(Figures{img, ""})
	}
	return e
}

func getPaiEvaluateCode(s *pythonExecutor, cl *ir.EvaluateStmt) (string, string, string, string, error) {
	// TODO(typhoonzero): Do **NOT** create tmp table when the select statement is like:
	// "SELECT fields,... FROM table"
	dbName, tableName, err := createTmpTableFromSelect(cl.Select, s.Session.DbConnStr)
	if err != nil {
		return "", "", "", "", err
	}
	cl.TmpEvaluateTable = strings.Join([]string{dbName, tableName}, ".")

	ossModelPath, e := getModelPath(cl.ModelName, s.Session)
	if e != nil {
		return "", "", "", "", e
	}

	currProject, err := database.GetDatabaseName(s.Session.DbConnStr)
	if err != nil {
		return "", "", "", "", err
	}
	modelType, estimator, err := getOSSSavedModelType(ossModelPath, currProject)
	if err != nil {
		return "", "", "", "", err
	}
	// format resultTable name to "db.table" to let the codegen form a submitting
	// argument of format "odps://project/tables/table_name"
	// PAIML do not need to create explain result manually, PAI will
	// create the result table.
	if cl.Into != "" && modelType != model.PAIML {
		resultTableParts := strings.Split(cl.Into, ".")
		if len(resultTableParts) == 1 {
			cl.Into = fmt.Sprintf("%s.%s", currProject, cl.Into)
		}
		db, err := database.OpenAndConnectDB(s.Session.DbConnStr)
		if err != nil {
			return "", "", "", "", err
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
			return "", "", "", "", err
		}
	}
	scriptPath := fmt.Sprintf("file://%s/%s", s.Cwd, tarball)
	paramsPath := fmt.Sprintf("file://%s/%s", s.Cwd, paramsFile)
	if err := createPAIHyperParamFile(s.Cwd, paramsFile, ossModelPath); err != nil {
		return "", "", "", "", err
	}
	code, paiCmd, requirements, e := pai.Evaluate(cl, s.Session, scriptPath, paramsPath, cl.ModelName, ossModelPath, s.Cwd, modelType)
	if e != nil {
		return "", "", "", "", e
	}
	return code, paiCmd, requirements, estimator, nil
}

func (s *paiExecutor) ExecuteEvaluate(cl *ir.EvaluateStmt) error {
	code, paiCmd, requirements, estimator, e := getPaiEvaluateCode(s.pythonExecutor, cl)
	defer dropTmpTables([]string{cl.TmpEvaluateTable}, s.Session.DbConnStr)
	if e != nil {
		return e
	}
	return s.submitPAITask(code, paiCmd, requirements, estimator)
}

func executeOptimizeUsingOptFlow(pythonExecutor *pythonExecutor, stmt *ir.OptimizeStmt) error {
	dbName, tableName, err := createTmpTableFromSelect(stmt.Select, pythonExecutor.Session.DbConnStr)
	if err != nil {
		return err
	}

	dropTmpTableFunc := func(table string) {
		dropTmpTables([]string{table}, pythonExecutor.Session.DbConnStr)
	}

	if len(stmt.Variables) > 2 {
		joinedVarName := strings.Join(stmt.Variables, "__")
		concatColumnNames := make([]string, 0)
		for i, v := range stmt.Variables {
			if i >= 1 {
				concatColumnNames = append(concatColumnNames, `','`)
			}
			concatColumnNames = append(concatColumnNames, v)
		}
		concatColumnExpr := fmt.Sprintf("CONCAT(%s) AS %s", strings.Join(concatColumnNames, ","), joinedVarName)
		selectStmt := fmt.Sprintf("SELECT *, %s FROM %s.%s", concatColumnExpr, dbName, tableName)
		newDBName, newTableName, err := createTmpTableFromSelect(selectStmt, pythonExecutor.Session.DbConnStr)
		dropTmpTableFunc(tableName) // drop the first created table whatever
		if err != nil {
			return err
		}
		stmt.Variables = []string{joinedVarName}
		dbName = newDBName
		tableName = newTableName
	}

	defer dropTmpTableFunc(tableName)

	db, err := database.OpenAndConnectDB(pythonExecutor.Session.DbConnStr)
	if err != nil {
		return err
	}
	defer db.Close()

	splittedResultTable := strings.SplitN(stmt.ResultTable, ".", 2)
	var resultTable string
	if len(splittedResultTable) == 2 {
		if splittedResultTable[0] != dbName {
			return fmt.Errorf("database name of result table must be the same as source table")
		}
		resultTable = stmt.ResultTable
	} else {
		resultTable = fmt.Sprintf("%s.%s", dbName, stmt.ResultTable)
	}

	_, err = db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", resultTable))
	if err != nil {
		return err
	}

	code, err := optimize.GenerateOptimizeCode(stmt, pythonExecutor.Session, tableName, true)
	if err != nil {
		return err
	}

	if err = pythonExecutor.runProgram(code, false); err != nil {
		return err
	}
	return nil
}

func (s *paiExecutor) ExecuteOptimize(stmt *ir.OptimizeStmt) error {
	return executeOptimizeUsingOptFlow(s.pythonExecutor, stmt)
}

func (s *paiExecutor) ExecuteRun(runStmt *ir.RunStmt) error {
	// For PAI submitter, we won't support `TO RUN`.
	// If we want to use `TO RUN` on MaxCompute, use alisa Submitter instead.
	return fmt.Errorf("ExecuteRun is not supported in PAI submitter")
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
	if err != nil {
		return
	}
	ret, err := bucket.IsObjectExist(modelName + "/tensorflow_model_desc")
	if err != nil {
		return
	}
	if ret {
		modelType = model.TENSORFLOW
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
		modelType = model.XGBOOST
		return
	}
	modelType = model.PAIML
	return
}

// deleteDirRecursive recursively delete a directory on the OSS
func deleteDirRecursive(bucket *oss.Bucket, dir string) error {
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
	if len(objectPathList) > 0 {
		_, err = bucket.DeleteObjects(objectPathList)
		if err != nil {
			return err
		}
	}
	return nil
}

func getCreateShapResultSQL(db *database.DB, tableName string, selectStmt string, labelCol string) (string, error) {
	// create table to record shap values for every feature for each sample.
	flds, _, err := verifier.GetSQLFieldType(selectStmt, db)
	if err != nil {
		return "", err
	}
	columnDefList := []string{}
	columnType := "STRING"
	if db.DriverName == "mysql" {
		columnType = "VARCHAR(255)"
	}
	for _, fieldName := range flds {
		if fieldName != labelCol {
			columnDefList = append(columnDefList, fmt.Sprintf("%s %s", fieldName, columnType))
		}
	}
	createStmt := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (%s);`, tableName, strings.Join(columnDefList, ","))
	return createStmt, nil
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
	// runtime and sqlflow_models are built-in packages.
	if err := copyPythonPackage("runtime", cwd); err != nil {
		return err
	}
	if err := copyPythonPackage("sqlflow_models", cwd); err != nil {
		return err
	}

	// add custom package if needed
	if err := copyCustomPackage(estimator, cwd); err != nil {
		return err
	}

	cmd := exec.Command("tar", "czf", tarball, "./runtime", "./sqlflow_models", entryFile, "requirements.txt")
	cmd.Dir = cwd
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed %s, %v", cmd, err)
	}
	return nil
}

func (s *paiExecutor) GetTrainStmtFromModel() bool { return false }

func pickPAILogViewerURL(output string) []string {
	return reODPSLogURL.FindAllString(output, -1)
}

func diagnose(taskType, output string) error {
	msg := fmt.Sprintf("%s task failed", taskType)
	if strings.Contains(output, ossAuthErrorMsg) {
		tips := os.Getenv("ERROR_PAI2OSS")
		if len(tips) == 0 {
			tips = "due to lack of the auth for PAI to access OSS(need to contact your administrator)"
		}
		msg = fmt.Sprintf("%s, %s", msg, tips)
	}
	lv := pickPAILogViewerURL(output)
	return fmt.Errorf("%s, please go to check details error logs in the LogViewer website: %s", msg, strings.Join(lv, "\n"))
}
