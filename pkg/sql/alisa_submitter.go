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

	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/pipe"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

type alisaSubmitter struct {
}

func (s *alisaSubmitter) ExecuteQuery(cl *ir.StandardSQL) error {
	return nil
}

func (s *alisaSubmitter) ExecuteTrain(cl *ir.TrainStmt) error {
	// TODO: implement submit train task:
	//
	// code := pai.Train(cl, cl.Into, pai.s.Cwd)
	// cmd.Execute("tar", "czf", "train.tar.gz", pai.s.Cwd)
	// cmd.Execute("osscmd", "cp", "train.tar.gz", "oss://sqlflow-bucket/...")
	// goalisa.createTask("pai -name tensorflow1120 -Dscript=@@train.tar.gz",
	// 	res_download_url={downloadUrl:"http://oss-endpoint/train.tar.gz", "resourceName":"train.tar.gz"})
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
