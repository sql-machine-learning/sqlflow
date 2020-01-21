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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"sqlflow.org/goalisa"
	"sqlflow.org/gomaxcompute"
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

func (s *alisaSubmitter) submitAlisaTask(code, resourceURL string) error {
	_, dsName, err := database.ParseURL(s.Session.DbConnStr)
	if err != nil {
		return err
	}
	cfg, e := goalisa.ParseDSN(dsName)
	if e != nil {
		return e
	}

	cfg.Env["RES_DOWNLOAD_URL"] = fmt.Sprintf(`[{\"downloadUrl\":\"%s\", \"resourceName\":\"%s\"}]`, resourceURL, resourceName)
	cfg.Verbose = true
	newDatasource := cfg.FormatDSN()

	alisa, e := database.OpenDB(fmt.Sprintf("alisa://%s", newDatasource))
	if e != nil {
		return e
	}
	_, e = alisa.Exec(code)
	return e
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

	if e = s.cleanUpModel(ossModelPath); e != nil {
		return e
	}
	// upload Alisa resource file to OSS
	ossObjectName := randStringRunes(16)
	// Alisa resource should be prefix with @@, alisa source would replace it with the RES_DOWN_URL.resourceName in alisa env.
	scriptPath := fmt.Sprintf("file://@@%s", resourceName)
	code, paiCmd, e := pai.Train(ts, s.Session, scriptPath, ts.Into, ossModelPath, s.Cwd)
	if e != nil {
		return e
	}
	return s.alisaCmd(code, paiCmd, ossObjectName)
}

func (s *alisaSubmitter) alisaCmd(program, alisaCode, ossObjectName string) error {
	tarball := "job.tar.gz"
	ep := os.Getenv("SQLFLOW_OSS_ALISA_ENDPOINT")
	ak := os.Getenv("SQLFLOW_OSS_AK")
	sk := os.Getenv("SQLFLOW_OSS_SK")
	bucketName := os.Getenv("SQLFLOW_OSS_ALISA_BUCKET")

	if ep == "" || ak == "" || sk == "" || bucketName == "" {
		return fmt.Errorf("should define SQLFLOW_OSS_ALISA_ENDPOINT, SQLFLOW_OSS_ALISA_BUCKET, SQLFLOW_OSS_AK, SQLFLOW_OSS_SK when using submitter alisa")
	}
	resourceURL := fmt.Sprintf("https://%s.%s/%s", bucketName, ep, ossObjectName)

	if e := achieveResource(s.Cwd, program, tarball); e != nil {
		return e
	}

	bucket, err := getBucket(ep, ak, sk, bucketName)
	if err != nil {
		return err
	}
	if e := bucket.PutObjectFromFile(ossObjectName, filepath.Join(s.Cwd, tarball)); e != nil {
		return err
	}
	defer bucket.DeleteObject(ossObjectName)

	return s.submitAlisaTask(alisaCode, resourceURL)
}

func (s *alisaSubmitter) cleanUpModel(modelPath string) error {
	ossCkptDir := os.Getenv("SQLFLOW_OSS_CHECKPOINT_DIR")
	sub := reOSS.FindStringSubmatch(ossCkptDir)
	if len(sub) != 3 {
		return fmt.Errorf("SQLFLOW_OSS_CHECKPOINT_DIR should be format: oss://bucket/?role_arn=xxx&host=xxx")
	}
	fmt.Println(os.Getenv("SQLFLOW_OSS_MODEL_ENDPOINT"), os.Getenv("SQLFLOW_OSS_AK"), os.Getenv("SQLFLOW_OSS_SK"), sub[1])
	bucket, e := getBucket(os.Getenv("SQLFLOW_OSS_MODEL_ENDPOINT"), os.Getenv("SQLFLOW_OSS_AK"), os.Getenv("SQLFLOW_OSS_SK"), sub[1])
	if e != nil {
		return e
	}
	fmt.Println("delete oss", modelPath)
	if e := bucket.DeleteObject(modelPath); e != nil {
		return e
	}
	return nil
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
	isDeepModel, e := ossModelFileExists(ossModelPath)
	if e != nil {
		return e
	}

	ossObjectName := randStringRunes(16)
	scriptPath := fmt.Sprintf("file://@@%s", resourceName)
	code, paiCmd, e := pai.Predict(ps, s.Session, scriptPath, ps.Using, ossModelPath, s.Cwd, isDeepModel)
	if e != nil {
		return e
	}
	return s.alisaCmd(code, paiCmd, ossObjectName)
}

func (s *alisaSubmitter) ExecuteExplain(cl *ir.ExplainStmt) error {
	return fmt.Errorf("Alisa submitter does not support EXPLAIN clause")
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

func getBucket(endpoint, ak, sk, bucketName string) (*oss.Bucket, error) {
	cli, err := oss.New(endpoint, ak, sk)
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
	driverName, dsName, e := database.ParseURL(session.DbConnStr)
	if e != nil {
		return "", e
	}
	userID := session.UserId
	var projectName string
	if driverName == "maxcompute" {
		cfg, e := gomaxcompute.ParseDSN(dsName)
		if e != nil {
			return "", e
		}
		projectName = cfg.Project
	} else if driverName == "alisa" {
		cfg, e := goalisa.ParseDSN(dsName)
		if e != nil {
			return "", e
		}
		projectName = cfg.Project
	}
	if userID == "" {
		userID = "unkown"
	}
	return strings.Join([]string{projectName, userID, modelName}, "/"), nil
}
