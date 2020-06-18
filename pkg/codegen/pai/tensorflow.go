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
	"strconv"
	"strings"
	"text/template"

	"sqlflow.org/sqlflow/pkg/codegen/tensorflow"
	"sqlflow.org/sqlflow/pkg/ir"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

func generateLoadOSSModelCode(estimator, ossModelPathToLoad string) (string, error) {
	if ossModelPathToLoad == "" {
		return "", nil
	}

	loadCodeTemplate := template.Must(template.New("LoadModel").Parse(tfLoadModelTmplText))
	filler := loadModelFiller{
		OSSModelDir: ossModelPathToLoad,
		Estimator:   estimator,
	}

	var loadCode bytes.Buffer
	if err := loadCodeTemplate.Execute(&loadCode, filler); err != nil {
		return "", err
	}

	return loadCode.String(), nil
}

// TFTrainWithLoadAndSave generates PAI-TF train program.
// Load pre-trained model if modelPathToLoad != "".
// Save the trained model to modelPathToSave.
func TFTrainWithLoadAndSave(ir *ir.TrainStmt, session *pb.Session, modelPathToSave, modelPathToLoad string, cc *ClusterConfig) (string, error) {
	// Distributed training must call train_and_evaluate, which need the user to specify validation.select
	valSelect, valOK := ir.Attributes["validation.select"]
	hasVal := true
	if !valOK || valSelect.(string) == "" {
		hasVal = false
	}
	if cc.Worker.Count > 1 && !hasVal {
		return "", fmt.Errorf("Distributed training must specify WITH validation.select")
	}

	loadCode, err := generateLoadOSSModelCode(ir.Estimator, modelPathToLoad)
	if err != nil {
		return "", err
	}

	trainCode, err := tensorflow.Train(ir, session)
	if err != nil {
		return "", err
	}

	// append code snippet to save model
	checkpointDir := OSSModelURL(modelPathToSave)
	var tpl = template.Must(template.New("SaveModel").Parse(tfSaveModelTmplText))
	filler := saveModelFiller{
		OSSModelDir: checkpointDir,
		Estimator:   ir.Estimator,
		NumWorkers:  cc.Worker.Count,
	}
	var saveCode bytes.Buffer
	if err = tpl.Execute(&saveCode, filler); err != nil {
		return "", err
	}

	fullCode := fmt.Sprintf("%s\n%s\n%s", loadCode, trainCode, saveCode.String())
	return fullCode, nil
}

// TFLoadAndPredict generates PAI-TF prediction program.
func TFLoadAndPredict(ir *ir.PredictStmt, session *pb.Session, modelPath string) (string, error) {
	var tpl = template.Must(template.New("Predict").Parse(tfPredictTmplText))
	ossModelDir := OSSModelURL(modelPath)
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
		Using:       ir.Using,
	}
	var code bytes.Buffer
	if err := tpl.Execute(&code, filler); err != nil {
		return "", err
	}
	return code.String(), nil
}

// TFLoadAndExplain generates PAI-TF explain program.
func TFLoadAndExplain(ir *ir.ExplainStmt, session *pb.Session, modelPath string, expn *ExplainRender) (string, error) {
	var tpl = template.Must(template.New("Explain").Parse(tfExplainTmplText))
	ossModelDir := OSSModelURL(modelPath)
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
		// TODO(weiguo): use GFile to write oss without ak/sk
		// ref: https://yuque.antfin-inc.com/pai-user/manual/tf_oss_by_gfile
		ResultOSSDest:     expn.key,
		ResultOSSAK:       expn.ak,
		ResultOSSSK:       expn.sk,
		ResultOSSEndpoint: expn.endpoint,
		ResultOSSBucket:   expn.bucket,
	}
	var code bytes.Buffer
	if err := tpl.Execute(&code, filler); err != nil {
		return "", err
	}
	return code.String(), nil
}

// TFLoadAndEvaluate generates PAI-TF evaluate program.
func TFLoadAndEvaluate(ir *ir.EvaluateStmt, session *pb.Session, modelPath string) (string, error) {
	var tpl = template.Must(template.New("Evaluate").Parse(tfEvaluateTmplText))
	ossModelDir := OSSModelURL(modelPath)
	paiExplainTable := ""
	if tensorflow.IsPAI() && ir.TmpEvaluateTable != "" {
		paiExplainTable = ir.TmpEvaluateTable
	}

	// set default metrics to "Accuracy"
	validationMetrics := "Accuracy"
	if v, ok := ir.Attributes["validationMetrics"]; ok {
		// validationMetrics should be a string like: "Accuracy,AUC,Recall"
		validationMetrics = v.(string)
	}

	filler := evaluateFiller{
		OSSModelDir:       ossModelDir,
		DataSource:        session.DbConnStr,
		Select:            ir.Select,
		ResultTable:       ir.Into,
		IsPAI:             tensorflow.IsPAI(),
		PAITable:          paiExplainTable,
		ValidationMetrics: validationMetrics,
	}
	var code bytes.Buffer
	if err := tpl.Execute(&code, filler); err != nil {
		return "", err
	}
	return code.String(), nil
}

func getTFPAICmd(cc *ClusterConfig, tarball, paramsFile, modelName, ossModelPath, trainTable, valTable, resTable, project, cwd string) (string, error) {
	jobName := strings.Replace(strings.Join([]string{"sqlflow", modelName}, "_"), ".", "_", 0)
	cfString, err := json.Marshal(cc)
	if err != nil {
		return "", err
	}
	cfQuote := strconv.Quote(string(cfString))

	// submit table should format as: odps://<project>/tables/<table>,odps://<project>/tables/<table>...
	submitTables, err := maxComputeTableURL(trainTable)
	if err != nil {
		return "", err
	}
	if trainTable != valTable && valTable != "" {
		valTable, err := maxComputeTableURL(valTable)
		if err != nil {
			return "", err
		}
		submitTables = fmt.Sprintf("%s,%s", submitTables, valTable)
	}
	outputTables := ""
	if resTable != "" {
		table, err := maxComputeTableURL(resTable)
		if err != nil {
			return "", err
		}
		outputTables = fmt.Sprintf("-Doutputs=%s", table)
	}

	// NOTE(typhoonzero): use -DhyperParameters to define flags passing OSS credentials.
	// TODO(typhoonzero): need to find a more secure way to pass credentials.
	cmd := fmt.Sprintf("pai -name tensorflow1150 -project algo_public_dev -DmaxHungTimeBeforeGCInSeconds=0 -DjobName=%s -Dtags=dnn -Dscript=%s -DentryFile=entry.py -Dtables=%s %s -DhyperParameters=\"%s\"",
		jobName, tarball, submitTables, outputTables, paramsFile)

	// format the oss checkpoint path with ARN authorization.
	ossCheckpointConfigs := os.Getenv("SQLFLOW_OSS_CHECKPOINT_DIR")
	if ossCheckpointConfigs == "" {
		return "", fmt.Errorf("need to configure SQLFLOW_OSS_CHECKPOINT_DIR when submitting to PAI")
	}
	ossJSONConfigs := make(map[string]string)
	if err := json.Unmarshal([]byte(ossCheckpointConfigs), &ossJSONConfigs); err != nil {
		return "", err
	}
	currProjectOSS, ok := ossJSONConfigs[project]
	if !ok {
		return "", fmt.Errorf("project %s not configured in SQLFLOW_OSS_CHECKPOINT_DIR", project)
	}
	arnSplited := strings.Split(currProjectOSS, "?")
	if len(arnSplited) != 2 {
		return "", fmt.Errorf("need to configure SQLFLOW_OSS_CHECKPOINT_DIR when submitting to PAI")
	}
	arn := arnSplited[1]
	ossURI := OSSModelURL(ossModelPath)
	ossCheckpointPath := fmt.Sprintf("%s/?%s", ossURI, arn)
	cmd = fmt.Sprintf("%s -DcheckpointDir='%s'", cmd, ossCheckpointPath)

	if cc.Worker.Count > 1 {
		cmd = fmt.Sprintf("%s -Dcluster=%s", cmd, cfQuote)
	} else {
		cmd = fmt.Sprintf("%s -DgpuRequired='%d'", cmd, cc.Worker.GPU)
	}
	return cmd, nil
}
