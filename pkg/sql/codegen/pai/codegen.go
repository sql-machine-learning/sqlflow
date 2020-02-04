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
	"log"
	"os"
	"strconv"
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
	// ModelTypeRandomForests is the model type that trained by PAI random forests.
	ModelTypeRandomForests
	// ModelTypeXGBoost is the model type that use PAI Tensorflow to train XGBoost models.
	ModelTypeXGBoost
)

const entryFile = "entry.py"

// PSConfig implicates Parameter Server Config
type PSConfig struct {
	Count int `json:"count"`
	GPU   int `json:"gpu"`
	CPU   int `json:"cpu"`
}

// WorkerConfig implicates Worker Config
type WorkerConfig struct {
	Count int `json:"count"`
	GPU   int `json:"gpu"`
	CPU   int `json:"cpu"`
}

// ClusterConfig implicates PAI distributed task meta
type ClusterConfig struct {
	PS     PSConfig     `json:"ps"`
	Worker WorkerConfig `json:"worker"`
}

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

// GetClusterConfig returns ClusterConfig object comes from WITH clause
func GetClusterConfig(attrs map[string]interface{}) (*ClusterConfig, error) {
	defaultMap := map[string]int{
		"train.num_ps":      0,
		"train.num_workers": 1,
		"train.worker_cpu":  400,
		"train.worker_gpu":  0,
		"train.ps_cpu":      200,
		"train.ps_gpu":      0,
	}
	for k := range defaultMap {
		attrValue, ok := attrs[k]
		if ok {
			intValue, intok := attrValue.(int)
			if !intok {
				return nil, fmt.Errorf("attribute %s must be int, got: %s", k, attrValue)
			}
			defaultMap[k] = intValue
			delete(attrs, k)
		}
	}
	return &ClusterConfig{
		PS: PSConfig{
			Count: defaultMap["train.num_ps"],
			CPU:   defaultMap["train.ps_cpu"],
			GPU:   defaultMap["train.ps_gpu"],
		},
		Worker: WorkerConfig{
			Count: defaultMap["train.num_workers"],
			CPU:   defaultMap["train.worker_cpu"],
			GPU:   defaultMap["train.worker_gpu"],
		},
	}, nil
}

func formatODPSTables(table string) (string, error) {
	parts := strings.Split(table, ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("odps table: %s should be format db.table", table)
	}
	return fmt.Sprintf("odps://%s/tables/%s", parts[0], parts[1]), nil
}

func getTFPAICmd(cc *ClusterConfig, tarball, modelName, ossModelPath, trainTable, valTable, resTable string) (string, error) {
	jobName := strings.Replace(strings.Join([]string{"sqlflow", modelName}, "_"), ".", "_", 0)
	cfString, err := json.Marshal(cc)
	if err != nil {
		return "", err
	}
	cfQuote := strconv.Quote(string(cfString))
	ckpDir, err := FormatCkptDir(ossModelPath)
	if err != nil {
		return "", err
	}

	// submit table should format as: odps://<project>/tables/<table>,odps://<project>/tables/<table>...
	submitTables, err := formatODPSTables(trainTable)
	if err != nil {
		return "", err
	}
	if trainTable != valTable && valTable != "" {
		valTable, err := formatODPSTables(valTable)
		if err != nil {
			return "", err
		}
		submitTables = fmt.Sprintf("%s,%s", submitTables, valTable)
	}
	outputTables := ""
	if resTable != "" {
		table, err := formatODPSTables(resTable)
		if err != nil {
			return "", err
		}
		outputTables = fmt.Sprintf("-Doutputs=%s", table)
	}
	if cc.Worker.Count > 1 {
		return fmt.Sprintf("pai -name tensorflow1120 -DjobName=%s -Dtags=dnn -Dscript=%s -DentryFile=entry.py -Dtables=%s %s -DcheckpointDir=\"%s\" -Dcluster=%s", jobName, tarball, submitTables, outputTables, ckpDir, cfQuote), nil
	}
	return fmt.Sprintf("pai -name tensorflow1120 -DjobName=%s -Dtags=dnn -DgpuRequired='' -Dscript=%s -DentryFile=entry.py -Dtables=%s %s -DcheckpointDir=\"%s\"", jobName, tarball, submitTables, outputTables, ckpDir), nil
}

func getTrainRandomForestsPAICmd(ir *ir.TrainStmt, session *pb.Session) (string, error) {
	// default use numTrees = 1
	treeNum := 1
	treeNumAttr, ok := ir.Attributes["tree_num"]
	if ok {
		treeNum = treeNumAttr.(int)
	}
	featureCols := []string{}
	for _, fclist := range ir.Features {
		for _, fc := range fclist {
			featureCols = append(featureCols, fc.GetFieldDesc()[0].Name)
		}
	}

	return fmt.Sprintf(`pai -name randomforests -DinputTableName="%s" -DmodelName="%s" -DlabelColName="%s" -DfeatureColNames="%s" -DtreeNum="%d"`,
		ir.TmpTrainTable, ir.Into, ir.Label.GetFieldDesc()[0].Name, strings.Join(featureCols, ","), treeNum), nil
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
	} else if strings.HasPrefix(strings.ToLower(ir.Estimator), "xgboost") {
		if code, e = xgboost.Train(ir, session); e != nil {
			return
		}
		var ossURI string
		if ossURI, e = FormatCkptDir(ossModelPath); e != nil {
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

// TFTrainAndSave generates PAI-TF train program.
func TFTrainAndSave(ir *ir.TrainStmt, session *pb.Session, modelPath string, cc *ClusterConfig) (string, error) {
	code, err := tensorflow.Train(ir, session)
	if err != nil {
		return "", err
	}

	// append code snippet to save model
	var tpl = template.Must(template.New("SaveModel").Parse(tfSaveModelTmplText))
	ckptDir, err := FormatCkptDir(modelPath)
	if err != nil {
		return "", err
	}
	filler := saveModelFiller{
		OSSModelDir: ckptDir,
		Estimator:   ir.Estimator,
		NumWorkers:  cc.Worker.Count,
	}
	var saveCode bytes.Buffer
	if err = tpl.Execute(&saveCode, filler); err != nil {
		return "", err
	}
	return code + saveCode.String(), nil
}

func getPredictRandomForestsPAICmd(ir *ir.PredictStmt, session *pb.Session) (string, error) {
	// NOTE(typhoonzero): for PAI random forests predicting, we can not load the TrainStmt
	// since the model saving is fully done by PAI. We directly use the columns in SELECT
	// statement for prediction, error will be reported by PAI job if the columns not match.
	db, err := database.OpenAndConnectDB(session.DbConnStr)
	if err != nil {
		return "", err
	}
	flds, _, err := getColumnTypes(ir.Select, db)
	if err != nil {
		return "", err
	}
	// drop result table if exists
	db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s;", ir.ResultTable))

	return fmt.Sprintf(`pai -name prediction -DmodelName="%s" -DinputTableName="%s" -DoutputTableName="%s" -DfeatureColNames="%s"`,
		ir.Using, ir.TmpPredictTable, ir.ResultTable, strings.Join(flds, ",")), nil
}

// Predict generates a Python program for predict data on PAI.
func Predict(ir *ir.PredictStmt, session *pb.Session, tarball, modelName, ossModelPath, cwd string, modelType int) (code, paiCmd, requirements string, e error) {
	if modelType == ModelTypeRandomForests {
		requirements, e = genRequirements(false)
		log.Printf("predicting using pai random forests")
		if paiCmd, e = getPredictRandomForestsPAICmd(ir, session); e != nil {
			return
		}
	} else if modelType == ModelTypeXGBoost {
		requirements, e = genRequirements(true)
		var ossURI string
		if ossURI, e = FormatCkptDir(ossModelPath); e != nil {
			return
		}
		var xgbPredCode bytes.Buffer
		var tpl = template.Must(template.New("xgbPredTemplate").Parse(xgbPredTemplateText))
		filler := &xgbPredictFiller{
			OSSModelDir:      ossURI,
			DataSource:       session.DbConnStr,
			PredSelect:       ir.Select,
			ResultTable:      ir.ResultTable,
			HDFSNameNodeAddr: session.HdfsNamenodeAddr,
			HiveLocation:     session.HiveLocation,
			HDFSUser:         session.HdfsUser,
			HDFSPass:         session.HdfsPass,
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

// TFLoadAndPredict generates PAI-TF prediction program.
func TFLoadAndPredict(ir *ir.PredictStmt, session *pb.Session, modelPath string) (string, error) {
	var tpl = template.Must(template.New("Predict").Parse(tfPredictTmplText))
	ossModelDir, err := FormatCkptDir(modelPath)
	if err != nil {
		return "", err
	}
	paiPredictTable := ""
	if tensorflow.IsPAI() && ir.TmpPredictTable != "" {
		paiPredictTable = ir.TmpPredictTable
	}
	filler := predictFiller{
		OSSModelDir: ossModelDir,
		DataSource:  session.DbConnStr,
		Select:      ir.Select,
		ResultTable: ir.ResultTable,
		IsPAI:       tensorflow.IsPAI(),
		PAITable:    paiPredictTable,
	}
	var code bytes.Buffer
	if err := tpl.Execute(&code, filler); err != nil {
		return "", err
	}
	return code.String(), nil
}

func getExplainRandomForestsPAICmd(ir *ir.ExplainStmt, session *pb.Session) (string, error) {
	// NOTE(typhoonzero): for PAI random forests predicting, we can not load the TrainStmt
	// since the model saving is fully done by PAI. We directly use the columns in SELECT
	// statement for prediction, error will be reported by PAI job if the columns not match.
	db, err := database.OpenAndConnectDB(session.DbConnStr)
	if err != nil {
		return "", err
	}
	flds, _, err := getColumnTypes(ir.Select, db)
	if err != nil {
		return "", err
	}
	// drop result table if exists
	db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s;", ir.Into))
	labelCol, ok := ir.Attributes["label_column"]
	if !ok {
		return "", fmt.Errorf("must specify WITH label_column when using pai random forest to explain models")
	}
	featureFileds := []string{}
	for _, f := range flds {
		if f != labelCol {
			featureFileds = append(featureFileds, f)
		}
	}
	return fmt.Sprintf(`pai -name feature_importance -project algo_public -DmodelName="%s" -DinputTableName="%s"  -DoutputTableName="%s" -DlabelColName="%s" -DfeatureColNames="%s"`,
		ir.ModelName, ir.TmpExplainTable, ir.Into, labelCol.(string), strings.Join(featureFileds, ",")), nil
}

// TFLoadAndExplain generates PAI-TF explain program.
func TFLoadAndExplain(ir *ir.ExplainStmt, session *pb.Session, modelPath string) (string, error) {
	var tpl = template.Must(template.New("Explain").Parse(tfExplainTmplText))
	ossModelDir, err := FormatCkptDir(modelPath)
	if err != nil {
		return "", err
	}
	paiExplainTable := ""
	if tensorflow.IsPAI() && ir.TmpExplainTable != "" {
		paiExplainTable = ir.TmpExplainTable
	}
	filler := explainFiller{
		OSSModelDir: ossModelDir,
		DataSource:  session.DbConnStr,
		Select:      ir.Select,
		ResultTable: ir.Into,
		IsPAI:       tensorflow.IsPAI(),
		PAITable:    paiExplainTable,
	}
	var code bytes.Buffer
	if err := tpl.Execute(&code, filler); err != nil {
		return "", err
	}
	return code.String(), nil
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
	if modelType == ModelTypeRandomForests {
		requirements, e = genRequirements(false)
		log.Printf("explain using pai random forests")
		if paiCmd, e = getExplainRandomForestsPAICmd(ir, session); e != nil {
			return
		}
	} else if modelType == ModelTypeXGBoost {
		requirements, e = genRequirements(true)
		log.Printf("explain using pai xgboost")
		var ossURI string
		if ossURI, e = FormatCkptDir(ossModelPath); e != nil {
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
