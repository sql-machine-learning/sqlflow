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

	"sqlflow.org/goalisa"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/sql/codegen/pai"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

type alisaSubmitter struct {
	*defaultSubmitter
}

func (s *alisaSubmitter) ExecuteQuery(sql *ir.StandardSQL) error {
	return runStandardSQL(s.Writer, string(*sql), s.Db)
}

func createTarball(files []string, target string) error {
	command := []string{"tar", "czf"}
	for _, fn := range files {
		command = append(command, fn)
	}
	command = append(command, target)
	cmd := exec.Command(command[0], command[1:]...)
	if _, err := cmd.CombinedOutput(); err != nil {
		fmt.Errorf("failed %s, %v", cmd, err)
	}
	return nil
}

func (s *alisaSubmitter) getPAIcmd(ts *ir.TrainStmt, tarball string) (string, error) {
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

	if cf.Worker.Count > 1 {
		return fmt.Sprintf("pai -name tensorflow1120 -DjobName=%s -Dtags=dnn -Dscript=file://@@%s -DentryFile=entry.py -Dtables=%s -DcheckpointDir=%s -Dcluster=%s", jobName, tarball, ts.TmpTrainTable, "", cfQuote), nil
	}
	return fmt.Sprintf("pai -name tensorflow1120 -DjobName=%s -Dtags=dnn -Dscript=file://@@%s -DentryFile=entry.py -Dtables=%s -DcheckpointDir=%s", jobName, tarball, ts.TmpTrainTable, "", cfQuote), nil
}

func (s *alisaSubmitter) submitAlisaTask(code, resourceName string) error {
	cfg, e := goalisa.ParseDSN(s.Session.DbConnStr)
	if e != nil {
		return e
	}

	cfg.Env["RES_DOWNLOAD_URL"] = fmt.Sprintf(`[{\"downloadUrl\":\"https://pai-tf-job.oss-cn-beijing.aliyuncs.com/%s\", \"resourceName\":\train.tar.gz\"}]`, resourceName)
	cfg.Verbose = true
	newDSN := cfg.FormatDSN()

	alisa, e := database.OpenDB(newDSN)
	if e != nil {
		return e
	}
	_, e = alisa.Exec(code)
	return e
}

func (s *alisaSubmitter) ExecuteTrain(cl *ir.TrainStmt) error {
	var e error
	cl.TmpTrainTable, cl.TmpValidateTable, e = createTempTrainAndValTable(cl.Select, cl.ValidationSelect, s.Session.DbConnStr)
	if e != nil {
		return e
	}
	defer dropTmpTables([]string{cl.TmpTrainTable, cl.TmpValidateTable}, s.Session.DbConnStr)

	code, e := pai.Train(cl, s.Session, cl.Into, s.Cwd)
	if e != nil {
		return e
	}

	f, err := os.Create(filepath.Join(s.Cwd, "entry.py"))
	if err != nil {
		return fmt.Errorf("create python code failed")
	}
	f.WriteString(code)
	defer f.Close()

	tarball := "train.tar.gz"
	if e := createTarball([]string{"$PYTHONPATH/sqlflow_submitter", "entry.py"}, tarball); e != nil {
		return e
	}

	// upload a temporary file to oss, used for alisa task
	// should fill the oss configuration in file `~/.ossutilconfig`
	resourceName := randStringRunes(16)
	cmd := exec.Command("ossutil", "cp", "train.tar.gz", fmt.Sprintf("oss://pai-tf/%s", resourceName))
	if _, e := cmd.CombinedOutput(); e != nil {
		return fmt.Errorf("failed %s, %v", cmd, e)
	}
	defer func() {
		exec.Command("ossutil", "rm", fmt.Sprintf("oss://pai-tf/%s", resourceName))
		cmd.CombinedOutput()
	}()

	paiCmd, e := s.getPAIcmd(cl, tarball)
	if e != nil {
		return e
	}

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
