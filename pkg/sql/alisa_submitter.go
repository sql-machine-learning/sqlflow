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
	"strconv"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"sqlflow.org/goalisa"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/sql/codegen/pai"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

var tarball = "train.tar.gz"
var entryFile = "entry.py"

type alisaSubmitter struct {
	*defaultSubmitter
}

func (s *alisaSubmitter) getPAIcmd(ts *ir.TrainStmt) (string, error) {
	cf, err := pai.GetClusterConfig(ts.Attributes)
	if err != nil {
		return "", err
	}

	jobName := strings.Replace(strings.Join([]string{"sqlflow", ts.Into}, "_"), ".", "_", 0)
	cfString, err := json.Marshal(cf)
	if err != nil {
		return "", err
	}
	cfQuote := strconv.Quote(string(cfString))
	ckpDir, err := pai.FormatCkptDir(ts.Into)
	if err != nil {
		return "", err
	}

	// submit table should format as: odps://<project>/tables/<table>,odps://<project>/tables/<table>...
	parts := strings.Split(ts.TmpTrainTable, ".")
	submitTables := fmt.Sprintf("odps://%s/tables/%s", parts[0], parts[1])
	if ts.TmpValidateTable != ts.TmpTrainTable {
		parts = strings.Split(ts.TmpValidateTable, ".")
		submitTables = fmt.Sprintf("%s,odps://%s/tables/%s", submitTables, parts[0], parts[1])
	}
	if cf.Worker.Count > 1 {
		return fmt.Sprintf("pai -name tensorflow1120 -DjobName=%s -Dtags=dnn -Dscript=file://@@%s -DentryFile=entry.py -Dtables=%s -DcheckpointDir=\"%s\" -Dcluster=%s", jobName, tarball, submitTables, ckpDir, cfQuote), nil
	}
	return fmt.Sprintf("pai -name tensorflow1120 -DjobName=%s -Dtags=dnn -Dscript=file://@@%s -DentryFile=entry.py -Dtables=%s -DcheckpointDir=\"%s\"", jobName, tarball, submitTables, ckpDir), nil
}

func (s *alisaSubmitter) submitAlisaTask(code, resourceName string) error {
	_, dSName, err := database.ParseURL(s.Session.DbConnStr)
	if err != nil {
		return err
	}
	cfg, e := goalisa.ParseDSN(dSName)
	if e != nil {
		return e
	}

	ossURL := os.Getenv("SQLFLOW_ALISA_OSS_HTTP_ENDPOINT")
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

func (s *alisaSubmitter) ExecuteTrain(cl *ir.TrainStmt) (e error) {
	cl.TmpTrainTable, cl.TmpValidateTable, e = createTempTrainAndValTable(cl.Select, cl.ValidationSelect, s.Session.DbConnStr)
	if e != nil {
		return e
	}
	defer dropTmpTables([]string{cl.TmpTrainTable, cl.TmpValidateTable}, s.Session.DbConnStr)

	paiCmd, e := s.getPAIcmd(cl)
	if e != nil {
		return e
	}

	code, e := pai.TFTrainAndSave(cl, s.Session, cl.Into)
	if e != nil {
		return e
	}

	if e = s.achieveResource(code, tarball); e != nil {
		return e
	}

	// upload Alisa resource file to OSS
	resourceName := randStringRunes(16)
	bucket, err := getBucket(os.Getenv("SQLFLOW_ALISA_OSS_BUCKET"))
	if err != nil {
		return err
	}
	if e = bucket.PutObjectFromFile(resourceName, filepath.Join(s.Cwd, tarball)); e != nil {
		return err
	}
	defer func() {
		bucket.DeleteObject(resourceName)
	}()

	return s.submitAlisaTask(paiCmd, resourceName)
}

func (s *alisaSubmitter) ExecutePredict(cl *ir.PredictStmt) error {
	return nil
}

func (s *alisaSubmitter) ExecuteExplain(cl *ir.ExplainStmt) error {
	return fmt.Errorf("Alisa submitter does not support EXPLAIN clause")
}

func (s *alisaSubmitter) GetTrainStmtFromModel() bool { return false }
func init()                                           { SubmitterRegister("alisa", &alisaSubmitter{&defaultSubmitter{}}) }

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
