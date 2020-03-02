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

package pai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"

	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/ir"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen/tensorflow"
	"sqlflow.org/sqlflow/pkg/sql/codegen/xgboost"
	"sqlflow.org/sqlflow/pkg/verifier"
)

const (
	// ModelTypeTF is the mode type that trained by PAI Tensorflow.
	ModelTypeTF = iota
	// ModelTypeXGBoost is the model type that use PAI Tensorflow to train XGBoost models.
	ModelTypeXGBoost
	// ModelTypePAIML is the model type that trained by PAI machine learing algorithm tookit
	ModelTypePAIML
)

const entryFile = "entry.py"

// GetOSSCheckpointDir from env SQLFLOW_OSS_CHECKPOINT_DIR for current project
func GetOSSCheckpointDir(project string) (string, error) {
	ossCheckpointDirMap := make(map[string]string)
	ossCheckpointEnv := os.Getenv("SQLFLOW_OSS_CHECKPOINT_DIR")
	if ossCheckpointEnv == "" {
		return "", fmt.Errorf("must set SQLFLOW_OSS_CHECKPOINT_DIR environment variable when training with PAI")
	}
	err := json.Unmarshal([]byte(ossCheckpointEnv), &ossCheckpointDirMap)
	if err != nil {
		return "", fmt.Errorf("SQLFLOW_OSS_CHECKPOINT_DIR should be a json string like {project: oss_dir, ...}: %v", err)
	}
	projectOSSConf, ok := ossCheckpointDirMap[project]
	if !ok {
		return "", fmt.Errorf("SQLFLOW_OSS_CHECKPOINT_DIR have no project key: %s", project)
	}
	return projectOSSConf, nil
}

// checkpointURL returns the saved model path on OSS
func checkpointURL(modelName string, project string) (string, error) {
	ossChekpointDir, err := GetOSSCheckpointDir(project)
	if err != nil {
		return "", err
	}
	ossURIParts := strings.Split(ossChekpointDir, "?")
	if len(ossURIParts) != 2 {
		return "", fmt.Errorf("SQLFLOW_OSS_CHECKPOINT_DIR must be of format: oss://bucket/?role_arn=xxx&host=xxx")
	}
	ossDir := strings.Join([]string{strings.TrimRight(ossURIParts[0], "/"), modelName}, "/")
	// Form URI like: oss://bucket/your/path/modelname/?args=...
	return strings.Join([]string{ossDir + "/", ossURIParts[1]}, "?"), nil
}

func maxComputeTableURL(table string) (string, error) {
	parts := strings.Split(table, ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("odps table: %s should be format db.table", table)
	}
	return fmt.Sprintf("odps://%s/tables/%s", parts[0], parts[1]), nil
}

// getColumnTypes is quiet like verify but accept a SQL string as input, and returns
// an ordered list of the field types.
// FIXME(typhoonzero): copied from executor_ir.go
func getColumnTypes(slct string, db *database.DB) ([]string, []string, error) {
	rows, err := db.Query(slct)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil, fmt.Errorf("query %s gives 0 row", slct)
	}

	if rows.Err() != nil {
		return nil, nil, err
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, nil, err
	}

	ft := []string{}
	flds := []string{}
	for _, ct := range columnTypes {
		_, fld := verifier.Decomp(ct.Name())
		typeName := ct.DatabaseTypeName()
		flds = append(flds, fld)
		ft = append(ft, typeName)
	}

	return flds, ft, nil
}

func genRequirements(isXGBoost bool) (string, error) {
	filler := requirementsFiller{
		IsXGBoost: isXGBoost,
	}
	var tpl = template.Must(template.New("requirements").Parse(paiRequirementsTmplText))
	var code bytes.Buffer
	if e := tpl.Execute(&code, filler); e != nil {
		return "", e
	}
	return code.String(), nil
}

// Train generates a Python program a PAI command arguments to train a Tensorflow model.
func Train(ir *ir.TrainStmt, session *pb.Session, tarball, modelName, ossModelPath, cwd string) (code, paiCmd, requirements string, e error) {
	cc, e := GetClusterConfig(ir.Attributes)
	if e != nil {
		return "", "", "", e
	}
	currProject, e := database.GetDatabaseName(session.DbConnStr)
	if e != nil {
		return "", "", "", e
	}
	if strings.ToLower(ir.Estimator) == "randomforests" {
		if paiCmd, e = getTrainRandomForestsPAICmd(ir, session); e != nil {
			return
		}
	} else if strings.ToLower(ir.Estimator) == "kmeans" {
		if paiCmd, e = getTrainKMeansPAICmd(ir, session); e != nil {
			return
		}
	} else if strings.HasPrefix(strings.ToLower(ir.Estimator), "xgboost") {
		if code, e = xgboost.Train(ir, session); e != nil {
			return
		}
		var ossURI string
		if ossURI, e = checkpointURL(ossModelPath, currProject); e != nil {
			return
		}
		var tpl = template.Must(template.New("xgbSaveModel").Parse(xgbSaveModelTmplText))
		var saveCode bytes.Buffer
		if e = tpl.Execute(&saveCode, &xgbSaveModelFiller{OSSModelDir: ossURI}); e != nil {
			return
		}
		code = code + saveCode.String()
		if cc.Worker.Count > 1 {
			return "", "", "", fmt.Errorf("when running xgboost on PAI, we only support run with one worker")
		}
		if paiCmd, e = getTFPAICmd(cc, tarball, modelName, ossModelPath, ir.TmpTrainTable, ir.TmpValidateTable, "", currProject); e != nil {
			return
		}
		requirements, e = genRequirements(true)
	} else {
		code, e = TFTrainAndSave(ir, session, ossModelPath, cc)
		if e != nil {
			return
		}
		if paiCmd, e = getTFPAICmd(cc, tarball, modelName, ossModelPath, ir.TmpTrainTable, ir.TmpValidateTable, "", currProject); e != nil {
			return
		}
		requirements, e = genRequirements(false)
	}
	return
}

// Predict generates a Python program for train a TensorFlow model.
func Predict(ir *ir.PredictStmt, session *pb.Session, tarball, modelName, ossModelPath, cwd string, modelType int) (code, paiCmd, requirements string, e error) {
	currProject := ""
	currProject, e = database.GetDatabaseName(session.DbConnStr)
	if e != nil {
		return
	}
	if modelType == ModelTypePAIML {
		if paiCmd, e = getPAIPredictCmd(ir, session); e != nil {
			return
		}
	} else if modelType == ModelTypeXGBoost {
		requirements, e = genRequirements(true)

		var ossURI string
		if ossURI, e = checkpointURL(ossModelPath, currProject); e != nil {
			return
		}
		var xgbPredCode bytes.Buffer
		var tpl = template.Must(template.New("xgbPredTemplate").Parse(xgbPredTemplateText))
		paiPredictTable := ""
		if tensorflow.IsPAI() && ir.TmpPredictTable != "" {
			paiPredictTable = ir.TmpPredictTable
		}
		filler := &xgbPredictFiller{
			OSSModelDir:      ossURI,
			DataSource:       session.DbConnStr,
			PredSelect:       ir.Select,
			ResultTable:      ir.ResultTable,
			HDFSNameNodeAddr: session.HdfsNamenodeAddr,
			HiveLocation:     session.HiveLocation,
			HDFSUser:         session.HdfsUser,
			HDFSPass:         session.HdfsPass,
			PAIPredictTable:  paiPredictTable,
		}
		if e = tpl.Execute(&xgbPredCode, filler); e != nil {
			return
		}
		code = xgbPredCode.String()

		cc, err := GetClusterConfig(ir.Attributes)
		if err != nil {
			return
		}
		// NOTE(typhoonzero): submit a PAI TF job to install xgboost and run.
		if paiCmd, e = getTFPAICmd(cc, tarball, modelName, ossModelPath, ir.TmpPredictTable, "", ir.ResultTable, currProject); e != nil {
			return
		}
	} else {
		requirements, e = genRequirements(false)
		cc, err := GetClusterConfig(ir.Attributes)
		if err != nil {
			return
		}
		if code, e = TFLoadAndPredict(ir, session, ossModelPath); e != nil {
			return
		}
		if paiCmd, e = getTFPAICmd(cc, tarball, modelName, ossModelPath, ir.TmpPredictTable, "", ir.ResultTable, currProject); e != nil {
			return
		}
	}
	return
}

// Explain generates a Python program for train a TensorFlow model.
func Explain(ir *ir.ExplainStmt, session *pb.Session, tarball, modelName, ossModelPath, cwd string, modelType int) (*ExplainRender, error) {
	cc, err := GetClusterConfig(ir.Attributes)
	if err != nil {
		return nil, err
	}
	currProject, err := database.GetDatabaseName(session.DbConnStr)
	if err != nil {
		return nil, err
	}

	expn := newExplainRender(session.UserId)
	if modelType == ModelTypePAIML {
		if ir.Into == "" {
			return nil, fmt.Errorf("explain PAI random forests model need INTO clause to output the explain result to a table")
		}
		if expn.Requirements, err = genRequirements(false); err != nil {
			return nil, err
		}
		expn.PaiCmd, err = getExplainRandomForestsPAICmd(ir, session)
	} else if modelType == ModelTypeXGBoost {
		if expn.Requirements, err = genRequirements(true); err != nil {
			return nil, err
		}
		ossURI, err := checkpointURL(ossModelPath, currProject)
		if err != nil {
			return nil, err
		}
		var xgbExplainCode bytes.Buffer
		var tpl = template.Must(template.New("xgbExplainTemplate").Parse(xgbExplainTemplateText))
		filler := &xgbExplainFiller{
			OSSModelDir:      ossURI,
			DataSource:       session.DbConnStr,
			DatasetSQL:       ir.Select,
			ResultTable:      ir.Into,
			IsPAI:            tensorflow.IsPAI(),
			PAIExplainTable:  ir.TmpExplainTable,
			HDFSNameNodeAddr: session.HdfsNamenodeAddr,
			HiveLocation:     session.HiveLocation,
			HDFSUser:         session.HdfsUser,
			HDFSPass:         session.HdfsPass,
			ResultOSSDest:    expn.key,
			// TODO(weiguo): use GFile to write oss without ak/sk
			// ref: https://yuque.antfin-inc.com/pai-user/manual/tf_oss_by_gfile
			ResultOSSAK:       expn.ak,
			ResultOSSSK:       expn.sk,
			ResultOSSEndpoint: expn.endpoint,
			ResultOSSBucket:   expn.bucket,
		}
		if err = tpl.Execute(&xgbExplainCode, filler); err != nil {
			return nil, err
		}
		expn.Code = xgbExplainCode.String()
		cc, err := GetClusterConfig(ir.Attributes)
		if err != nil {
			return nil, err
		}
		// NOTE(typhoonzero): submit a PAI TF job to install xgboost and run.
		expn.PaiCmd, err = getTFPAICmd(cc, tarball, modelName, ossModelPath, ir.TmpExplainTable, "", ir.Into, currProject)
	} else {
		if expn.Requirements, err = genRequirements(false); err != nil {
			return nil, err
		}
		// run explain PAI TF
		if expn.Code, err = TFLoadAndExplain(ir, session, ossModelPath, expn); err != nil {
			return expn, err
		}
		expn.PaiCmd, err = getTFPAICmd(cc, tarball, modelName, ossModelPath, ir.TmpExplainTable, "", ir.Into, currProject)
	}
	return expn, err
}
