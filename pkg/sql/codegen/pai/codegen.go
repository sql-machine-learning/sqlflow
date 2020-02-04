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
	"sqlflow.org/sqlflow/pkg/sql/codegen/xgboost"
	"sqlflow.org/sqlflow/pkg/verifier"
)

const entryFile = "entry.py"

// FormatCkptDir returns the saved model path on OSS
func FormatCkptDir(modelName string) (string, error) {
	ossCkptDir := os.Getenv("SQLFLOW_OSS_CHECKPOINT_DIR")
	if ossCkptDir == "" {
		return "", fmt.Errorf("must specify SQLFLOW_OSS_CHECKPOINT_DIR when training with PAI, e.g. oss://bucket/?role_arn=xxx&host=xxx")
	}
	ossURIParts := strings.Split(ossCkptDir, "?") // ossCkptDir: oss://bucket/your/path/?args=...
	if len(ossURIParts) != 2 {
		return "", fmt.Errorf("SQLFLOW_OSS_CHECKPOINT_DIR must be of format: oss://bucket/?role_arn=xxx&host=xxx")
	}
	ossDir := strings.Join([]string{strings.TrimRight(ossURIParts[0], "/"), modelName}, "/")
	// Form URI like: oss://bucket/your/path/modelname/?args=...
	return strings.Join([]string{ossDir + "/", ossURIParts[1]}, "?"), nil
}

func formatODPSTables(table string) (string, error) {
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
		requirements, e = genRequirements(false)
	} else if strings.HasPrefix(strings.ToLower(ir.Estimator), "xgboost") {
		if code, e = xgboost.Train(ir, session); e != nil {
			return
		}
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
func Predict(ir *ir.PredictStmt, session *pb.Session, tarball, modelName, ossModelPath, cwd string, isDeepModel bool) (code, paiCmd, requirements string, e error) {
	if !isDeepModel {
		log.Printf("predicting using pai random forests")
		if paiCmd, e = getPredictRandomForestsPAICmd(ir, session); e != nil {
			return
		}
	} else {
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
	requirements, e = genRequirements(false)
	return
}

// Explain generates a Python program for train a TensorFlow model.
func Explain(ir *ir.ExplainStmt, session *pb.Session, tarball, modelName, ossModelPath, cwd string, isDeepModel bool) (code, paiCmd, requirements string, e error) {
	if ir.Into == "" {
		return "", "", "", fmt.Errorf("explain PAI random forests model need INTO clause to output the explain result to a table")
	}
	cc, err := GetClusterConfig(ir.Attributes)
	if err != nil {
		return "", "", "", err
	}
	if !isDeepModel {
		log.Printf("predicting using pai random forests")
		if paiCmd, e = getExplainRandomForestsPAICmd(ir, session); e != nil {
			return
		}
	} else {
		// run explain PAI TF
		if code, e = TFLoadAndExplain(ir, session, modelName); e != nil {
			return
		}
		if paiCmd, e = getTFPAICmd(cc, tarball, modelName, ossModelPath, ir.TmpExplainTable, "", ir.Into); e != nil {
			return
		}
	}
	requirements, e = genRequirements(false)
	return
}
