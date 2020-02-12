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
	"fmt"
	"log"
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

// checkpointURL returns the saved model path on OSS
func checkpointURL(modelName string) (string, error) {
	ossURIParts := strings.Split(os.Getenv("SQLFLOW_OSS_CHECKPOINT_DIR"), "?")
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
		if ossURI, e = checkpointURL(ossModelPath); e != nil {
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
		if paiCmd, e = getTFPAICmd(cc, tarball, modelName, ossModelPath, ir.TmpTrainTable, ir.TmpValidateTable, ""); e != nil {
			return
		}
		requirements, e = genRequirements(true)
	} else {
		code, e = TFTrainAndSave(ir, session, ossModelPath, cc)
		if e != nil {
			return
		}
		if paiCmd, e = getTFPAICmd(cc, tarball, modelName, ossModelPath, ir.TmpTrainTable, ir.TmpValidateTable, ""); e != nil {
			return
		}
		requirements, e = genRequirements(false)
	}
	return
}

// Predict generates a Python program for train a TensorFlow model.
func Predict(ir *ir.PredictStmt, session *pb.Session, tarball, modelName, ossModelPath, cwd string, modelType int) (code, paiCmd, requirements string, e error) {
	if modelType == ModelTypePAIML {
		log.Printf("predicting using pai prediction tookit")
		if paiCmd, e = getPAIPredictCmd(ir, session); e != nil {
			return
		}
	} else if modelType == ModelTypeXGBoost {
		requirements, e = genRequirements(true)
		var ossURI string
		if ossURI, e = checkpointURL(ossModelPath); e != nil {
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
		if paiCmd, e = getTFPAICmd(cc, tarball, modelName, ossModelPath, ir.TmpPredictTable, "", ir.ResultTable); e != nil {
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
		if paiCmd, e = getTFPAICmd(cc, tarball, modelName, ossModelPath, ir.TmpPredictTable, "", ir.ResultTable); e != nil {
			return
		}
	}
	return
}

// Explain generates a Python program for train a TensorFlow model.
func Explain(ir *ir.ExplainStmt, session *pb.Session, tarball, modelName, ossModelPath, cwd string, modelType int) (code, paiCmd, requirements string, e error) {
	if ir.Into == "" {
		return "", "", "", fmt.Errorf("explain PAI random forests model need INTO clause to output the explain result to a table")
	}
	cc, err := GetClusterConfig(ir.Attributes)
	if err != nil {
		return "", "", "", err
	}
	if modelType == ModelTypePAIML {
		requirements, e = genRequirements(false)
		log.Printf("explain using pai random forests")
		if paiCmd, e = getExplainRandomForestsPAICmd(ir, session); e != nil {
			return
		}
	} else if modelType == ModelTypeXGBoost {
		requirements, e = genRequirements(true)
		log.Printf("explain using pai xgboost")
		var ossURI string
		if ossURI, e = checkpointURL(ossModelPath); e != nil {
			return
		}
		var xgbPredCode bytes.Buffer
		var tpl = template.Must(template.New("xgbExplainTemplate").Parse(xgbExplainTemplateText))
		filler := &xgbExplainFiller{
			OSSModelDir:      ossURI,
			DataSource:       session.DbConnStr,
			DatasetSQL:       ir.Select,
			ResultTable:      ir.Into,
			HDFSNameNodeAddr: session.HdfsNamenodeAddr,
			HiveLocation:     session.HiveLocation,
			HDFSUser:         session.HdfsUser,
			HDFSPass:         session.HdfsPass,
		}
		if e = tpl.Execute(&xgbPredCode, filler); e != nil {
			return
		}
		code = xgbPredCode.String()

		var cc *ClusterConfig
		if cc, e = GetClusterConfig(ir.Attributes); e != nil {
			return
		}
		// NOTE(typhoonzero): submit a PAI TF job to install xgboost and run.
		if paiCmd, e = getTFPAICmd(cc, tarball, modelName, ossModelPath, ir.TmpExplainTable, "", ir.Into); e != nil {
			return
		}
	} else {
		requirements, e = genRequirements(false)
		// run explain PAI TF
		if code, e = TFLoadAndExplain(ir, session, ossModelPath); e != nil {
			return
		}
		if paiCmd, e = getTFPAICmd(cc, tarball, modelName, ossModelPath, ir.TmpExplainTable, "", ir.Into); e != nil {
			return
		}
	}
	return
}
