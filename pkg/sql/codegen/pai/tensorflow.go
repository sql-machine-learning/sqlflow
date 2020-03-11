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
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"text/template"

	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/ir"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen/tensorflow"
)

// TFTrainAndSave generates PAI-TF train program.
func TFTrainAndSave(ir *ir.TrainStmt, session *pb.Session, modelPath string, cc *ClusterConfig) (string, error) {
	// Distributed training must call train_and_evaluate, which need the user to specify validation.select
	valSelect, valOK := ir.Attributes["validation.select"]
	hasVal := true
	if !valOK || valSelect.(string) == "" {
		hasVal = false
	}
	if cc.Worker.Count > 1 && !hasVal {
		return "", fmt.Errorf("Distributed training must specify WITH validation.select")
	}
	currProject, err := database.GetDatabaseName(session.DbConnStr)
	if err != nil {
		return "", err
	}
	ckptDir := ossModelURL(modelPath, currProject)
	code, err := tensorflow.Train(ir, session)
	if err != nil {
		return "", err
	}

	// append code snippet to save model
	var tpl = template.Must(template.New("SaveModel").Parse(tfSaveModelTmplText))
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

// TFLoadAndPredict generates PAI-TF prediction program.
func TFLoadAndPredict(ir *ir.PredictStmt, session *pb.Session, modelPath string) (string, error) {
	var tpl = template.Must(template.New("Predict").Parse(tfPredictTmplText))
	currProject, err := database.GetDatabaseName(session.DbConnStr)
	if err != nil {
		return "", err
	}
	ossModelDir := ossModelURL(modelPath, currProject)
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
	currProject, err := database.GetDatabaseName(session.DbConnStr)
	if err != nil {
		return "", err
	}
	ossModelDir := ossModelURL(modelPath, currProject)
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

func getTFPAICmd(cc *ClusterConfig, tarball, modelName, ossModelPath, trainTable, valTable, resTable, project, cwd string) (string, error) {
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
	// temp files under cwd will be cleaned after the job is finished.
	tmpfile, err := ioutil.TempFile(cwd, "sqlflow-paitemp-")

	ossAk := os.Getenv("SQLFLOW_OSS_AK")
	ossSk := os.Getenv("SQLFLOW_OSS_SK")
	ossEp := os.Getenv("SQLFLOW_OSS_MODEL_ENDPOINT")

	hdfsDir := fmt.Sprintf("%s/%s",
		strings.TrimRight(os.Getenv("SQLFLOW_HDFS_MODEL_CKPT_DIR"), "/"),
		strings.TrimLeft(ossModelPath, "/"))

	if _, err := tmpfile.Write([]byte(fmt.Sprintf("sqlflow_oss_ak=\"%s\"\n", ossAk))); err != nil {
		return "", err
	}
	if _, err := tmpfile.Write([]byte(fmt.Sprintf("sqlflow_oss_sk=\"%s\"\n", ossSk))); err != nil {
		return "", err
	}
	if _, err := tmpfile.Write([]byte(fmt.Sprintf("sqlflow_oss_ep=\"%s\"\n", ossEp))); err != nil {
		return "", err
	}
	ossModelURL := ossModelURL(ossModelPath, project)
	if _, err := tmpfile.Write([]byte(fmt.Sprintf("sqlflow_oss_modeldir=\"%s\"\n", ossModelURL))); err != nil {
		return "", err
	}
	if _, err := tmpfile.Write([]byte(fmt.Sprintf("sqlflow_hdfs_ckpt=\"%s\"\n", hdfsDir))); err != nil {
		return "", err
	}
	if err := tmpfile.Close(); err != nil {
		return "", err
	}

	// NOTE(typhoonzero): use -DhyperParameters to define flags passing OSS credentials.
	// TODO(typhoonzero): need to find a more secure way to pass credentials.
	cmd := fmt.Sprintf("pai -name tensorflow1150 -project algo_public_dev -DmaxHungTimeBeforeGCInSeconds=0 -DjobName=%s -Dtags=dnn -Dscript=%s -DentryFile=entry.py -Dtables=%s %s -DhyperParameters=\"file://%s\"",
		jobName, tarball, submitTables, outputTables, tmpfile.Name())
	if cc.Worker.Count > 1 {
		cmd = fmt.Sprintf("%s -Dcluster=%s", cmd, cfQuote)
	} else {
		cmd = fmt.Sprintf("%s -DgpuRequired='%d'", cmd, cc.Worker.GPU)
	}
	return cmd, nil
}
