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
	"bufio"
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

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"sqlflow.org/goalisa"
	"sqlflow.org/sqlflow/go/codegen/pai"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
	"sqlflow.org/sqlflow/go/randstring"
)

const (
	resourceName = "job.tar.gz"
	entryFile    = "entry.py"
)

const (
	// AlisaTaskTypePAI is PAI task in the Alisa task enumeration
	AlisaTaskTypePAI = iota
	// AlisaTaskTypePyODPS is PyODPS task in the Alisa task enumeration
	AlisaTaskTypePyODPS
)

var reOSS = regexp.MustCompile(`oss://([^/]+).*host=([^&]+)`)

type alisaExecutor struct {
	*pythonExecutor
}

type alisaTaskContext struct {
	codeResourceURL   string
	paramsResourceURL string
}

func (s *alisaExecutor) submitAlisaTask(taskType int, submitCode string, args string, context *alisaTaskContext) error {
	_, dsName, err := database.ParseURL(s.Session.DbConnStr)
	if err != nil {
		return err
	}
	cfg, e := goalisa.ParseDSN(dsName)
	if e != nil {
		return e
	}

	if taskType == AlisaTaskTypePAI {
		cfg.Env["RES_DOWNLOAD_URL"] = fmt.Sprintf(`[{"downloadUrl":"%s", "resourceName":"%s"}, {"downloadUrl":"%s", "resourceName":"%s"}]`,
			context.codeResourceURL, resourceName, context.paramsResourceURL, paramsFile)
	}
	cfg.Verbose = true
	alisa := goalisa.New(cfg)
	var b bytes.Buffer
	w := io.MultiWriter(os.Stdout, &b)

	switch taskType {
	case AlisaTaskTypePAI:
		if e := alisa.ExecWithWriter(submitCode, w); e != nil {
			return diagnose("PAI", b.String())
		}
	case AlisaTaskTypePyODPS:
		if e := alisa.ExecPyODPSWithWriter(submitCode, args, w); e != nil {
			return diagnose("PyODPS", b.String())
		}
	default:
		return fmt.Errorf("Unknown AlisaTaskType %d", taskType)
	}

	return nil
}

func (s *alisaExecutor) ExecuteTrain(ts *ir.TrainStmt) (e error) {
	if e = preExecuteTrainOnPAI(ts, s.Session); e != nil {
		return e
	}
	defer dropTmpTables([]string{ts.TmpTrainTable, ts.TmpValidateTable}, s.Session.DbConnStr)

	ossModelPathToSave, e := getModelPath(ts.Into, s.Session)
	if e != nil {
		return e
	}
	currProject, e := database.GetDatabaseName(s.Session.DbConnStr)
	if e != nil {
		return e
	}
	// cleanup saved model on OSS before training
	modelBucket, e := getModelBucket(currProject)
	if e != nil {
		return e
	}
	if e := deleteDirRecursive(modelBucket, ossModelPathToSave+"/"); e != nil {
		return e
	}

	// Alisa resource should be prefix with @@, alisa source would replace it with the RES_DOWN_URL.resourceName in alisa env.
	scriptPath := fmt.Sprintf("file://@@%s", resourceName)
	if e = pai.CleanupPAIModel(ts, s.Session); e != nil {
		return e
	}
	paramsPath := fmt.Sprintf("file://@@%s", paramsFile)
	if err := createPAIHyperParamFile(s.Cwd, paramsFile, ossModelPathToSave); err != nil {
		return err
	}

	code, paiCmd, requirements, e := pai.Train(ts, s.Session, scriptPath, paramsPath, ts.Into, ossModelPathToSave, ts.PreTrainedModel, s.Cwd)
	if e != nil {
		return e
	}
	// upload generated program to OSS and submit an Alisa task.
	return s.uploadResourceAndSubmitAlisaTask(code, requirements, paiCmd, ts.Estimator)
}

func (s *alisaExecutor) ExecutePredict(ps *ir.PredictStmt) error {
	dbName, tableName, err := createTmpTableFromSelect(ps.Select, s.Session.DbConnStr)
	if err != nil {
		return err
	}
	ps.TmpPredictTable = strings.Join([]string{dbName, tableName}, ".")
	defer dropTmpTables([]string{ps.TmpPredictTable}, s.Session.DbConnStr)

	if e := createPredictionResultTable(ps, s.Db, s.Session); e != nil {
		return e
	}

	ossModelPath, e := getModelPath(ps.Using, s.Session)
	if e != nil {
		return e
	}
	// NOTE(typhoonzero): current project may differ from the project from SELECT statement.
	currProject, e := database.GetDatabaseName(s.Session.DbConnStr)
	if e != nil {
		return e
	}
	modelType, estimator, e := getOSSSavedModelType(ossModelPath, currProject)
	if e != nil {
		return e
	}

	scriptPath := fmt.Sprintf("file://@@%s", resourceName)
	paramsPath := fmt.Sprintf("file://@@%s", paramsFile)
	if err := createPAIHyperParamFile(s.Cwd, paramsFile, ossModelPath); err != nil {
		return err
	}
	code, paiCmd, requirements, e := pai.Predict(ps, s.Session, scriptPath, paramsPath, ps.Using, ossModelPath, s.Cwd, modelType)
	if e != nil {
		return e
	}
	return s.uploadResourceAndSubmitAlisaTask(code, requirements, paiCmd, estimator)
}

func (s *alisaExecutor) uploadResourceAndSubmitAlisaTask(entryCode, requirements, alisaExecCode, estimator string) error {
	// upload generated program to OSS and submit an Alisa task.
	ossCodeObjectName := randstring.Generate(16)
	alisaBucket, e := getAlisaBucket()
	if e != nil {
		return e
	}
	codeResourceURL, e := tarAndUploadResource(s.Cwd, entryCode, requirements, ossCodeObjectName, estimator, alisaBucket)
	if e != nil {
		return e
	}
	defer alisaBucket.DeleteObject(ossCodeObjectName)
	// upload params.txt for additional training parameters.
	ossParamsObjectName := randstring.Generate(16)
	paramResourceURL, e := uploadResource(s.Cwd, paramsFile, ossParamsObjectName, alisaBucket)
	defer alisaBucket.DeleteObject(ossParamsObjectName)

	return s.submitAlisaTask(
		AlisaTaskTypePAI,
		alisaExecCode,
		``,
		&alisaTaskContext{
			codeResourceURL:   codeResourceURL,
			paramsResourceURL: paramResourceURL,
		})
}

func (s *alisaExecutor) ExecuteExplain(cl *ir.ExplainStmt) error {
	dbName, tableName, err := createTmpTableFromSelect(cl.Select, s.Session.DbConnStr)
	if err != nil {
		return err
	}
	cl.TmpExplainTable = strings.Join([]string{dbName, tableName}, ".")
	defer dropTmpTables([]string{cl.TmpExplainTable}, s.Session.DbConnStr)

	currProject, err := database.GetDatabaseName(s.Session.DbConnStr)
	if err != nil {
		return err
	}
	ossModelPath, e := getModelPath(cl.ModelName, s.Session)
	if e != nil {
		return e
	}
	modelType, estimator, e := getOSSSavedModelType(ossModelPath, currProject)
	if e != nil {
		return e
	}
	if cl.Into != "" {
		if e := createExplainResultTable(s.Db, cl, cl.Into, modelType, estimator); e != nil {
			return e
		}
	}

	scriptPath := fmt.Sprintf("file://@@%s", resourceName)
	paramsPath := fmt.Sprintf("file://@@%s", paramsFile)
	if err := createPAIHyperParamFile(s.Cwd, paramsFile, ossModelPath); err != nil {
		return err
	}
	expn, e := pai.Explain(cl, s.Session, scriptPath, paramsPath, cl.ModelName, ossModelPath, s.Cwd, modelType)
	if e != nil {
		return e
	}
	if e = s.uploadResourceAndSubmitAlisaTask(expn.Code, expn.Requirements, expn.PaiCmd, estimator); e != nil {
		return e
	}
	if img, e := expn.Draw(); e == nil {
		s.Writer.Write(Figures{img, ""})
	}
	return e
}

func (s *alisaExecutor) ExecuteEvaluate(es *ir.EvaluateStmt) error {
	dbName, tableName, e := createTmpTableFromSelect(es.Select, s.Session.DbConnStr)
	if e != nil {
		return e
	}
	es.TmpEvaluateTable = strings.Join([]string{dbName, tableName}, ".")
	defer dropTmpTables([]string{es.TmpEvaluateTable}, s.Session.DbConnStr)

	// default always output evaluation loss
	metricNames := []string{"loss"}
	metricsAttr, ok := es.Attributes["validation.metrics"]
	if ok {
		metricsList := strings.Split(metricsAttr.(string), ",")
		metricNames = append(metricNames, metricsList...)
	}
	if e = createEvaluationResultTable(s.Db, es.Into, metricNames); e != nil {
		return e
	}

	ossModelPath, e := getModelPath(es.ModelName, s.Session)
	if e != nil {
		return e
	}
	// NOTE(typhoonzero): current project may differ from the project from SELECT statement.
	currProject, e := database.GetDatabaseName(s.Session.DbConnStr)
	if e != nil {
		return e
	}
	modelType, estimator, e := getOSSSavedModelType(ossModelPath, currProject)
	if e != nil {
		return e
	}

	scriptPath := fmt.Sprintf("file://@@%s", resourceName)
	paramsPath := fmt.Sprintf("file://@@%s", paramsFile)
	if e = createPAIHyperParamFile(s.Cwd, paramsFile, ossModelPath); e != nil {
		return e
	}
	code, paiCmd, requirements, e := pai.Evaluate(es, s.Session, scriptPath, paramsPath, es.ModelName, ossModelPath, s.Cwd, modelType)
	if e != nil {
		return e
	}
	return s.uploadResourceAndSubmitAlisaTask(code, requirements, paiCmd, estimator)
}

func (s *alisaExecutor) ExecuteOptimize(stmt *ir.OptimizeStmt) error {
	return executeOptimizeUsingOptFlow(s.pythonExecutor, stmt)
}

func getRunnableProgramAbsPath(fileNameOrPath string) string {
	if path.IsAbs(fileNameOrPath) {
		return fileNameOrPath
	}

	return path.Join(sqlflowToRunProgramFolder, fileNameOrPath)
}

func (s *alisaExecutor) ExecuteRun(runStmt *ir.RunStmt) error {
	if len(runStmt.Parameters) == 0 {
		return fmt.Errorf("Parameters shouldn't be empty")
	}

	program := runStmt.Parameters[0]
	fileExtension := filepath.Ext(program)

	// If the first parameter is a Python program
	if strings.EqualFold(fileExtension, ".py") {
		programAbsPath := getRunnableProgramAbsPath(program)
		if _, e := os.Stat(programAbsPath); e != nil {
			return fmt.Errorf("Cannot find the Python file %s", programAbsPath)
		}

		// Build the arguments
		args := runStmt.Parameters[1:]
		args = append(args, fmt.Sprintf(`%s='%s'`, sqlflowToRunContextKeySelect, runStmt.Select))
		args = append(args, fmt.Sprintf(`%s='%s'`, sqlflowToRunContextKeyInto, runStmt.Into))
		args = append(args, fmt.Sprintf(`%s='%s'`, sqlflowToRunContextKeyImage, runStmt.ImageName))

		// Read the content of Python program
		code, e := ioutil.ReadFile(programAbsPath)
		if e != nil {
			return e
		}

		// Submit a PyODPS task
		return s.submitAlisaTask(AlisaTaskTypePyODPS, string(code), strings.Join(args, " "), nil)
	}

	return fmt.Errorf("Alisa executor only supports Python program but cannot execute %s", program)
}

func (s *alisaExecutor) GetTrainStmtFromModel() bool { return false }

func findPyModulePath(pyModuleName string) (string, error) {
	var b bytes.Buffer
	wStdout := bufio.NewWriter(&b)
	cmd := exec.Command("python", "-c", fmt.Sprintf(`import %s;print(%s.__path__[0])`, pyModuleName, pyModuleName))
	cmd.Stdout = wStdout
	if e := cmd.Run(); e != nil {
		return "", fmt.Errorf("failed %s, %v", cmd, e)
	}
	return strings.TrimSpace(b.String()), nil
}

// FIXME(typhoonzero): use the same model bucket name e.g. sqlflow-models
func getModelBucket(project string) (*oss.Bucket, error) {
	ak := os.Getenv("SQLFLOW_OSS_AK")
	sk := os.Getenv("SQLFLOW_OSS_SK")
	ep := os.Getenv("SQLFLOW_OSS_MODEL_ENDPOINT")
	if ak == "" || sk == "" || ep == "" {
		return nil, fmt.Errorf("should define SQLFLOW_OSS_MODEL_ENDPOINT, SQLFLOW_OSS_AK, SQLFLOW_OSS_SK when using submitter alisa")
	}

	cli, e := oss.New(ep, ak, sk)
	if e != nil {
		return nil, e
	}
	return cli.Bucket(pai.BucketName)
}

func getAlisaBucket() (*oss.Bucket, error) {
	ep := os.Getenv("SQLFLOW_OSS_ALISA_ENDPOINT")
	ak := os.Getenv("SQLFLOW_OSS_AK")
	sk := os.Getenv("SQLFLOW_OSS_SK")
	bucketName := os.Getenv("SQLFLOW_OSS_ALISA_BUCKET")

	if ep == "" || ak == "" || sk == "" {
		return nil, fmt.Errorf("should define SQLFLOW_OSS_ALISA_ENDPOINT, SQLFLOW_OSS_ALISA_BUCKET, SQLFLOW_OSS_AK, SQLFLOW_OSS_SK when using submitter alisa")
	}

	cli, err := oss.New(ep, ak, sk)
	if err != nil {
		return nil, err
	}
	return cli.Bucket(bucketName)
}

func writeFile(filePath, program string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("create python code failed")
	}
	defer f.Close()
	f.WriteString(program)
	return nil
}

func getModelPath(modelName string, session *pb.Session) (string, error) {
	userID := session.UserId
	projectName, err := database.GetDatabaseName(session.DbConnStr)
	if err != nil {
		return "", err
	}
	if userID == "" {
		userID = "unknown"
	}
	return strings.Join([]string{projectName, userID, modelName}, "/"), nil
}

func tarAndUploadResource(cwd, entryCode, requirements, ossObjectName, estimator string, bucket *oss.Bucket) (string, error) {
	if e := achieveResource(cwd, entryCode, requirements, tarball, estimator); e != nil {
		return "", e
	}
	return uploadResource(cwd, tarball, ossObjectName, bucket)
}

func uploadResource(cwd, localFileName, ossObjectName string, bucket *oss.Bucket) (string, error) {
	resourceURL := fmt.Sprintf("https://%s.%s/%s", bucket.BucketName, bucket.Client.Config.Endpoint, ossObjectName)
	if e := bucket.PutObjectFromFile(ossObjectName, filepath.Join(cwd, localFileName)); e != nil {
		return "", e
	}
	return resourceURL, nil
}
