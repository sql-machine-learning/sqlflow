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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"sqlflow.org/goalisa"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/sql/codegen/pai"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

var tarball = "task.tar.gz"
var entryFile = "entry.py"
var reCkpBucket = regexp.MustCompile(`oss://([^/]+)`)

type alisaSubmitter struct {
	*defaultSubmitter
}

func (s *alisaSubmitter) submitAlisaTask(code, resourceName string) error {
	_, dsName, err := database.ParseURL(s.Session.DbConnStr)
	if err != nil {
		return err
	}
	cfg, e := goalisa.ParseDSN(dsName)
	if e != nil {
		return e
	}

	ossURL := fmt.Sprintf("https://%s.%s", os.Getenv("SQLFLOW_ALISA_OSS_BUCKET"), os.Getenv("SQLFLOW_ALISA_OSS_ENDPOINT"))
	cfg.Env["RES_DOWNLOAD_URL"] = fmt.Sprintf(`[{\"downloadUrl\":\"%s/%s\", \"resourceName\":\"%s\"}]`, ossURL, resourceName, tarball)
	cfg.Verbose = true
	newDatasource := cfg.FormatDSN()

	alisa, e := database.OpenDB(fmt.Sprintf("alisa://%s", newDatasource))
	if e != nil {
		return e
	}
	_, e = alisa.Exec(code)
	return e
}

func (s *alisaSubmitter) getModelPath(modelName string) (string, error) {
	_, dsName, err := database.ParseURL(s.Session.DbConnStr)
	if err != nil {
		return "", err
	}
	cfg, err := goalisa.ParseDSN(dsName)
	if err != nil {
		return "", err
	}
	userID := s.Session.UserId
	if userID == "" {
		userID = "unkown"
	}
	return strings.Join([]string{cfg.Project, userID, modelName}, "/"), nil
}

func (s *alisaSubmitter) ExecuteTrain(ts *ir.TrainStmt) (e error) {
	ts.TmpTrainTable, ts.TmpValidateTable, e = createTempTrainAndValTable(ts.Select, ts.ValidationSelect, s.Session.DbConnStr)
	if e != nil {
		return e
	}
	defer dropTmpTables([]string{ts.TmpTrainTable, ts.TmpValidateTable}, s.Session.DbConnStr)

	cc, e := pai.GetClusterConfig(ts.Attributes)
	if e != nil {
		return e
	}

	modelPath, e := s.getModelPath(ts.Into)
	if e != nil {
		return e
	}

	paiCmd, e := getPAIcmd(cc, ts.Into, modelPath, ts.TmpTrainTable, ts.TmpValidateTable, "")
	if e != nil {
		return e
	}

	code, e := pai.TFTrainAndSave(ts, s.Session, modelPath, cc)
	if e != nil {
		return e
	}

	if e := s.cleanUpModel(modelPath); e != nil {
		return e
	}

	return s.submit(code, paiCmd)
}

func (s *alisaSubmitter) submit(program, alisaCode string) error {
	if e := s.achieveResource(program, tarball); e != nil {
		return e
	}

	// upload Alisa resource file to OSS
	resourceName := randStringRunes(16)
	bucket, err := getBucket(os.Getenv("SQLFLOW_ALISA_OSS_BUCKET"))
	if err != nil {
		return err
	}
	if e := bucket.PutObjectFromFile(resourceName, filepath.Join(s.Cwd, tarball)); e != nil {
		return err
	}
	defer bucket.DeleteObject(resourceName)

	return s.submitAlisaTask(alisaCode, resourceName)
}

func (s *alisaSubmitter) cleanUpModel(modelPath string) error {
	ossCkptDir := os.Getenv("SQLFLOW_OSS_CHECKPOINT_DIR")
	sub := reCkpBucket.FindStringSubmatch(ossCkptDir)
	if len(sub) != 2 {
		return fmt.Errorf("SQLFLOW_OSS_CHECKPOINT_DIR should be format: oss://bucket/?role_arn=xxx&host=xxx")
	}
	bucket, e := getBucket(sub[1])
	if e != nil {
		return e
	}
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

	cc, e := pai.GetClusterConfig(ps.Attributes)
	if e != nil {
		return e
	}
	modelPath, e := s.getModelPath(ps.Using)
	if e != nil {
		return e
	}
	paiCmd, e := getPAIcmd(cc, ps.Using, modelPath, ps.TmpPredictTable, "", ps.ResultTable)
	if e != nil {
		return e
	}
	code, e := pai.TFLoadAndPredict(ps, s.Session, modelPath)
	if e != nil {
		return e
	}
	return s.submit(code, paiCmd)
}

func (s *alisaSubmitter) ExecuteExplain(cl *ir.ExplainStmt) error {
	return fmt.Errorf("Alisa submitter does not support EXPLAIN clause")
}

func (s *alisaSubmitter) GetTrainStmtFromModel() bool { return false }

func (s *alisaSubmitter) achieveResource(entryCode, tarball string) error {
	if err := writeFile(filepath.Join(s.Cwd, entryFile), entryCode); err != nil {
		return err
	}

	path, err := findPyModulePath("sqlflow_submitter")
	if err != nil {
		return err
	}
	cmd := exec.Command("cp", "-r", path, ".")
	cmd.Dir = s.Cwd
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed %s, %v", cmd, err)
	}

	cmd = exec.Command("tar", "czf", tarball, "./sqlflow_submitter", entryFile)
	cmd.Dir = s.Cwd
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed %s, %v", cmd, err)
	}
	return nil
}

func findPyModulePath(pyModuleName string) (string, error) {
	cmd := exec.Command("python", "-c", fmt.Sprintf(`import %s;print(%s.__path__[0])`, pyModuleName, pyModuleName))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed %s, %v", cmd, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func getBucket(bucketName string) (*oss.Bucket, error) {
	cli, err := oss.New(os.Getenv("SQLFLOW_ALISA_OSS_ENDPOINT"), os.Getenv("SQLFLOW_ALISA_OSS_AK"), os.Getenv("SQLFLOW_ALISA_OSS_SK"))
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

func odpsTables(table string) (string, error) {
	parts := strings.Split(table, ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("odps table: %s should be format db.table", table)
	}
	return fmt.Sprintf("odps://%s/tables/%s", parts[0], parts[1]), nil
}

func getPAIcmd(cc *pai.ClusterConfig, modelName, ossModelPath, trainTable, valTable, resTable string) (string, error) {
	jobName := strings.Replace(strings.Join([]string{"sqlflow", modelName}, "_"), ".", "_", 0)
	cfString, err := json.Marshal(cc)
	if err != nil {
		return "", err
	}
	cfQuote := strconv.Quote(string(cfString))
	ckpDir, err := pai.FormatCkptDir(ossModelPath)
	if err != nil {
		return "", err
	}

	// submit table should format as: odps://<project>/tables/<table>,odps://<project>/tables/<table>...
	submitTables, err := odpsTables(trainTable)
	if err != nil {
		return "", err
	}
	if trainTable != valTable && valTable != "" {
		valTable, err := odpsTables(valTable)
		if err != nil {
			return "", err
		}
		submitTables = fmt.Sprintf("%s,%s", submitTables, valTable)
	}
	outputTables := ""
	if resTable != "" {
		table, err := odpsTables(resTable)
		if err != nil {
			return "", err
		}
		outputTables = fmt.Sprintf("-Doutputs=%s", table)
	}
	if cc.Worker.Count > 1 {
		return fmt.Sprintf("pai -name tensorflow1120 -DjobName=%s -Dtags=dnn -Dscript=file://@@%s -DentryFile=entry.py -Dtables=%s %s -DcheckpointDir=\"%s\" -Dcluster=%s", jobName, tarball, submitTables, outputTables, ckpDir, cfQuote), nil
	}
	return fmt.Sprintf("pai -name tensorflow1120 -DjobName=%s -Dtags=dnn -Dscript=file://@@%s -DentryFile=entry.py -Dtables=%s %s -DcheckpointDir=\"%s\"", jobName, tarball, submitTables, outputTables, ckpDir), nil
}
