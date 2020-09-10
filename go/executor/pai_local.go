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
	"fmt"

	"sqlflow.org/sqlflow/go/codegen/pai"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/ir"
	"sqlflow.org/sqlflow/go/model"
)

type paiLocalExecutor struct{ *pythonExecutor }

const setLocalFlagsCodeTmpl = `import os
from runtime.pai.pai_distributed import define_tf_flags
FLAGS = define_tf_flags()
FLAGS.sqlflow_oss_ak = os.getenv("SQLFLOW_OSS_AK")
FLAGS.sqlflow_oss_sk = os.getenv("SQLFLOW_OSS_SK")
FLAGS.sqlflow_oss_ep = os.getenv("SQLFLOW_OSS_MODEL_ENDPOINT")
FLAGS.sqlflow_oss_modeldir = "%[1]s"
FLAGS.checkpointDir = "."
`

func (s *paiLocalExecutor) ExecuteTrain(trainStmt *ir.TrainStmt) error {
	code, _, _, err := getPaiTrainCode(s.pythonExecutor, trainStmt)
	defer dropTmpTables([]string{trainStmt.TmpTrainTable, trainStmt.TmpValidateTable}, s.Session.DbConnStr)
	if err != nil {
		return err
	}
	ossModelPathToSave, e := getModelPath(trainStmt.Into, s.Session)
	if e != nil {
		return e
	}
	currProject, e := database.GetDatabaseName(s.Session.DbConnStr)
	if e != nil {
		return e
	}
	setLocalFlagsCode := fmt.Sprintf(setLocalFlagsCodeTmpl, pai.OSSModelURL(ossModelPathToSave))
	err = s.runProgram(setLocalFlagsCode+code, true)
	if err != nil {
		return err
	}
	// download model from OSS to local cwd and save to sqlfs
	// NOTE(typhoonzero): model in sqlfs will be used by sqlflow model zoo currently
	// should use the model in sqlfs when predicting.
	if e = downloadOSSModel(ossModelPathToSave+"/", currProject); e != nil {
		return e
	}
	m := model.New(s.Cwd, trainStmt.OriginalSQL)
	return m.Save(trainStmt.Into, s.Session)
}

func (s *paiLocalExecutor) ExecutePredict(predStmt *ir.PredictStmt) error {
	code, _, _, _, err := getPaiPredictCode(s.pythonExecutor, predStmt)

	defer dropTmpTables([]string{predStmt.TmpPredictTable}, s.Session.DbConnStr)
	if err != nil {
		return err
	}
	ossModelPathToSave, e := getModelPath(predStmt.Using, s.Session)
	if e != nil {
		return e
	}
	setLocalFlagsCode := fmt.Sprintf(setLocalFlagsCodeTmpl, pai.OSSModelURL(ossModelPathToSave))
	return s.runProgram(setLocalFlagsCode+code, true)
}

func (s *paiLocalExecutor) ExecuteExplain(explainStmt *ir.ExplainStmt) error {
	expn, _, err := getPaiExplainCode(s.pythonExecutor, explainStmt)
	defer dropTmpTables([]string{explainStmt.TmpExplainTable}, s.Session.DbConnStr)
	if err != nil {
		return err
	}
	ossModelPathToSave, e := getModelPath(explainStmt.ModelName, s.Session)
	if e != nil {
		return e
	}
	setLocalFlagsCode := fmt.Sprintf(setLocalFlagsCodeTmpl, pai.OSSModelURL(ossModelPathToSave))
	return s.runProgram(setLocalFlagsCode+expn.Code, true)
}

func (s *paiLocalExecutor) ExecuteEvaluate(evalStmt *ir.EvaluateStmt) error {
	code, _, _, _, err := getPaiEvaluateCode(s.pythonExecutor, evalStmt)
	defer dropTmpTables([]string{evalStmt.TmpEvaluateTable}, s.Session.DbConnStr)
	if err != nil {
		return err
	}
	ossModelPathToSave, e := getModelPath(evalStmt.ModelName, s.Session)
	if e != nil {
		return e
	}
	setLocalFlagsCode := fmt.Sprintf(setLocalFlagsCodeTmpl, pai.OSSModelURL(ossModelPathToSave))
	return s.runProgram(setLocalFlagsCode+code, true)
}
