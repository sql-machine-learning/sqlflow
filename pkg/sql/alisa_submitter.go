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

	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/pipe"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen/pai"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

type alisaSubmitter struct {
	*defaultSubmitter
}

func (s *alisaSubmitter) ExecuteQuery(cl *ir.StandardSQL) error {
	return nil
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
		return fmt.Sprintf("pai -name tensorflow1120 -DjobName=%s -Dtags=dnn -Dscript=file://%s -DentryFile=entry.py -Dtables=%s -DcheckpointDir=%s -Dcluster=%s", jobName, tarball, ts.TmpTrainTable, "", cfQuote), nil
	}
	return fmt.Sprintf("pai -name tensorflow1120 -DjobName=%s -Dtags=dnn -Dscript=file://%s -DentryFile=entry.py -Dtables=%s -DcheckpointDir=%s", jobName, tarball, ts.TmpTrainTable, "", cfQuote), nil
}

func (s *alisaSubmitter) ExecuteTrain(cl *ir.TrainStmt) error {
	// TODO(typhoonzero): Do **NOT** create tmp table when the select statement is like:
	// "SELECT fields,... FROM table"
	dbName, tableName, err := createTmpTableFromSelect(cl.Select, s.Session.DbConnStr)
	if err != nil {
		return err
	}
	cl.TmpTrainTable = strings.Join([]string{dbName, tableName}, ".")
	if cl.ValidationSelect != "" {
		dbName, tableName, err := createTmpTableFromSelect(cl.ValidationSelect, s.Session.DbConnStr)
		if err != nil {
			return err
		}
		cl.TmpValidateTable = strings.Join([]string{dbName, tableName}, ".")
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
	cmd := exec.Command("ossutil", "cp", "train.tar.gz", "oss://pai-tf/train.tar.gz")
	if _, e := cmd.CombinedOutput(); e != nil {
		return fmt.Errorf("failed %s, %v", cmd, e)
	}
	defer func() {
		exec.Command("ossutil", "rm", "oss://pai-tf/train.tar.gz")
		cmd.CombinedOutput()
	}()

	//TODO(yancey1989): call goalisa to create task and print logs
	/**
		code, err := s.getPAIcmd(cl, tarball)
		if err != nil {
			return err
		}
		alisa := goalisa.NewAlisaFromEnv()
		taskID, status, err := alisa.CreateTask(paiCmd)
	**/
	return nil
}

func (s *alisaSubmitter) ExecutePredict(cl *ir.PredictStmt) error {
	return nil
}

func (s *alisaSubmitter) ExecuteExplain(cl *ir.ExplainStmt) error {
	return fmt.Errorf("Alisa submitter does not support EXPLAIN clause")
}

func (s *alisaSubmitter) Setup(w *pipe.Writer, db *database.DB, modelDir string, cwd string, session *pb.Session) {
}

func (s *alisaSubmitter) GetTrainStmtFromModel() bool { return false }
func init()                                           { SubmitterRegister("alisa", &alisaSubmitter{}) }
