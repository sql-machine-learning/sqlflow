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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"sqlflow.org/goalisa"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/ir"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen/pai"
)

var resourceName = "job.tar.gz"
var entryFile = "entry.py"
var reOSS = regexp.MustCompile(`oss://([^/]+).*host=([^&]+)`)

type alisaSubmitter struct {
	*defaultSubmitter
}

func (s *alisaSubmitter) submitAlisaTask(submitCode, codeResourceURL, paramsResourceURL string) error {
	_, dsName, err := database.ParseURL(s.Session.DbConnStr)
	if err != nil {
		return err
	}
	cfg, e := goalisa.ParseDSN(dsName)
	if e != nil {
		return e
	}

	cfg.Env["RES_DOWNLOAD_URL"] = fmt.Sprintf(`[{"downloadUrl":"%s", "resourceName":"%s"}, {"downloadUrl":"%s", "resourceName":"%s"}]`,
		codeResourceURL, resourceName, paramsResourceURL, paramsFile)
	cfg.Verbose = true
	alisa := goalisa.New(cfg)
	var b bytes.Buffer
	w := io.MultiWriter(os.Stdout, &b)
	if e := alisa.ExecWithWriter(submitCode, w); e != nil {
		return fmt.Errorf("PAI task failed, please go to check details error logs in the LogViewer website: %s", strings.Join(pickPAILogViewerURL(b.String()), "\n"))
	}
	return nil

}

func (s *alisaSubmitter) ExecuteTrain(ts *ir.TrainStmt) (e error) {
	ts.TmpTrainTable, ts.TmpValidateTable, e = createTempTrainAndValTable(ts.Select, ts.ValidationSelect, s.Session.DbConnStr)
	if e != nil {
		return e
	}
	defer dropTmpTables([]string{ts.TmpTrainTable, ts.TmpValidateTable}, s.Session.DbConnStr)

	ossModelPath, e := getModelPath(ts.Into, s.Session)
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
	if e := deleteDirRecursive(modelBucket, ossModelPath+"/"); e != nil {
		return e
	}

	// Alisa resource should be prefix with @@, alisa source would replace it with the RES_DOWN_URL.resourceName in alisa env.
	scriptPath := fmt.Sprintf("file://@@%s", resourceName)
	if e = pai.CleanupPAIModel(ts, s.Session); e != nil {
		return e
	}
	paramsPath := fmt.Sprintf("file://@@%s", paramsFile)
	if err := createPAIHyperParamFile(s.Cwd, paramsFile, ossModelPath); err != nil {
		return err
	}
	code, paiCmd, requirements, e := pai.Train(ts, s.Session, scriptPath, paramsPath, ts.Into, ossModelPath, s.Cwd)
	if e != nil {
		return e
	}
	// upload generated program to OSS and submit an Alisa task.
	return s.uploadResourceAndSubmitAlisaTask(code, requirements, paiCmd)
}

func (s *alisaSubmitter) ExecutePredict(ps *ir.PredictStmt) error {
	dbName, tableName, err := createTmpTableFromSelect(ps.Select, s.Session.DbConnStr)
	if err != nil {
		return err
	}
	ps.TmpPredictTable = strings.Join([]string{dbName, tableName}, ".")
	defer dropTmpTables([]string{ps.TmpPredictTable}, s.Session.DbConnStr)

	if e := createPredictionTableFromIR(ps, s.Db, s.Session); e != nil {
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
	modelType, _, e := getOSSSavedModelType(ossModelPath, currProject)
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
	return s.uploadResourceAndSubmitAlisaTask(code, requirements, paiCmd)
}

func (s *alisaSubmitter) uploadResourceAndSubmitAlisaTask(entryCode, requirements, alisaExecCode string) error {
	// upload generated program to OSS and submit an Alisa task.
	ossCodeObjectName := randStringRunes(16)
	alisaBucket, e := getAlisaBucket()
	if e != nil {
		return e
	}
	codeResourceURL, e := tarAndUploadResource(s.Cwd, entryCode, requirements, ossCodeObjectName, alisaBucket)
	if e != nil {
		return e
	}
	defer alisaBucket.DeleteObject(ossCodeObjectName)
	// upload params.txt for additional training parameters.
	ossParamsObjectName := randStringRunes(16)
	paramResourceURL, e := uploadResource(s.Cwd, paramsFile, ossParamsObjectName, alisaBucket)
	defer alisaBucket.DeleteObject(ossParamsObjectName)

	return s.submitAlisaTask(alisaExecCode, codeResourceURL, paramResourceURL)
}

func (s *alisaSubmitter) ExecuteExplain(cl *ir.ExplainStmt) error {
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
	if e = s.uploadResourceAndSubmitAlisaTask(expn.Code, expn.Requirements, expn.PaiCmd); e != nil {
		return e
	}
	if img, e := expn.Draw(); e == nil {
		s.Writer.Write(Figures{img, ""})
	}
	return e
}

func (s *alisaSubmitter) GetTrainStmtFromModel() bool { return false }

func findPyModulePath(pyModuleName string) (string, error) {
	cmd := exec.Command("python", "-c", fmt.Sprintf(`import %s;print(%s.__path__[0])`, pyModuleName, pyModuleName))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed %s, %v", cmd, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// FIXME(typhoonzero): use the same model bucket name e.g. sqlflow-models
func getModelBucket(project string) (*oss.Bucket, error) {
	ak := os.Getenv("SQLFLOW_OSS_AK")
	sk := os.Getenv("SQLFLOW_OSS_SK")
	ep := os.Getenv("SQLFLOW_OSS_MODEL_ENDPOINT")
	if ak == "" || sk == "" || ep == "" {
		return nil, fmt.Errorf("should define SQLFLOW_OSS_MODEL_ENDPOINT, SQLFLOW_OSS_CHECKPOINT_DIR, SQLFLOW_OSS_AK, SQLFLOW_OSS_SK when using submitter alisa")
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

func tarAndUploadResource(cwd, entryCode, requirements, ossObjectName string, bucket *oss.Bucket) (string, error) {
	if e := achieveResource(cwd, entryCode, requirements, tarball); e != nil {
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
