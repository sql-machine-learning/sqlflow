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
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"sqlflow.org/sqlflow/pkg/database"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen/tensorflow"
	"sqlflow.org/sqlflow/pkg/sql/ir"
	"sqlflow.org/sqlflow/pkg/verifier"
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
	ossCkptDir = strings.Join([]string{ossDir + "/", ossURIParts[1]}, "?")
	return ossCkptDir, nil
}

// wrapper generates a Python program for submit TensorFlow tasks to PAI.
func wrapper(code, dataSource, modelName, cwd, tmpTrainTable, tmpValTable string, resultTable string, cc *ClusterConfig) (string, error) {
	f, err := os.Create(filepath.Join(cwd, entryFile))
	if err != nil {
		return "", fmt.Errorf("Create python code failed")
	}
	f.WriteString(code)
	f.Close()

	if err != nil {
		return "", err
	}
	ossCkptDir, err := FormatCkptDir(modelName)
	if err != nil {
		return "", err
	}

	var tpl = template.Must(template.New("Submit").Parse(tfWrapperTmplText))
	cfString, err := json.Marshal(cc)
	if err != nil {
		return "", err
	}
	isDistributed := false
	if cc.Worker.Count > 1 {
		isDistributed = true
	}
	filler := wrapperFiller{
		ClusterConfigJSON: strconv.Quote(string(cfString)),
		IsDistributed:     isDistributed,
		DataSource:        dataSource,
		ModelName:         modelName,
		EntryFile:         entryFile,
		PAITrainTable:     tmpTrainTable,
		PAIValidateTable:  tmpValTable,
		ResultTable:       resultTable,
		OSSCheckpointDir:  ossCkptDir,
	}
	var program bytes.Buffer
	if err := tpl.Execute(&program, filler); err != nil {
		return "", err
	}
	return program.String(), nil
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

func trainRandomForests(ir *ir.TrainStmt, session *pb.Session) (string, error) {
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
	filler := &randomForestsTrainFiller{
		DataSource:     session.DbConnStr,
		TmpTrainTable:  ir.TmpTrainTable,
		FeatureColumns: featureCols,
		LabelColumn:    ir.Label.GetFieldDesc()[0].Name,
		Save:           ir.Into,
		TreeNum:        treeNum,
	}
	var tpl = template.Must(template.New("RandomForestsTrain").Parse(randomForestsTrainTemplate))
	var rfCode bytes.Buffer
	if err := tpl.Execute(&rfCode, filler); err != nil {
		return "", err
	}
	return rfCode.String(), nil
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

// Train generates a Python program for train a TensorFlow model.
func Train(ir *ir.TrainStmt, session *pb.Session, modelName, cwd string) (string, error) {
	if strings.ToLower(ir.Estimator) == "randomforests" {
		return trainRandomForests(ir, session)
	}
	cc, err := GetClusterConfig(ir.Attributes)
	if err != nil {
		return "", err
	}
	program, err := TFTrainAndSave(ir, session, modelName, cc)
	if err != nil {
		return "", err
	}
	return wrapper(program, session.DbConnStr, modelName, cwd,
		ir.TmpTrainTable, ir.TmpValidateTable, "", cc)
}

// TFTrainAndSave generates PAI-TF train program.
func TFTrainAndSave(ir *ir.TrainStmt, session *pb.Session, modelName string, cc *ClusterConfig) (string, error) {
	code, err := tensorflow.Train(ir, session)
	if err != nil {
		return "", err
	}

	// append code snippet to save model
	var tpl = template.Must(template.New("SaveModel").Parse(tfSaveModelTmplText))
	ckptDir, err := FormatCkptDir(ir.Into)
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

func ossFileExists(modelName string) (bool, error) {
	endpoint := os.Getenv("SQLFLOW_OSS_ENDPOINT")
	ak := os.Getenv("SQLFLOW_OSS_AK")
	sk := os.Getenv("SQLFLOW_OSS_SK")
	// NOTE(typhoonzero): PAI Tensorflow need SQLFLOW_OSS_CHECKPOINT_DIR, get bucket name from it
	ossCheckpointDir := os.Getenv("SQLFLOW_OSS_CHECKPOINT_DIR")
	ckptParts := strings.Split(ossCheckpointDir, "?")
	if len(ckptParts) != 2 {
		return false, fmt.Errorf("SQLFLOW_OSS_CHECKPOINT_DIR got wrong format")
	}
	urlParts := strings.Split(ckptParts[0], "://")
	if len(urlParts) != 2 {
		return false, fmt.Errorf("SQLFLOW_OSS_CHECKPOINT_DIR got wrong format")
	}
	bucketName := strings.Split(urlParts[1], "/")[0]

	cli, err := oss.New(endpoint, ak, sk)
	if err != nil {
		return false, err
	}
	bucket, err := cli.Bucket(bucketName)
	if err != nil {
		return false, err
	}
	return bucket.IsObjectExist(modelName + "/sqlflow_model_desc")
}

func predictRandomForests(ir *ir.PredictStmt, session *pb.Session) (string, error) {
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
	filler := &randomForestsPredictFiller{
		DataSource:      session.DbConnStr,
		TmpPredictTable: ir.TmpPredictTable,
		FeatureColumns:  flds,
		Save:            ir.Using,
		ResultTable:     ir.ResultTable,
	}
	var tpl = template.Must(template.New("RandomForestsPredict").Parse(randomForestsPredictTemplate))
	var rfCode bytes.Buffer
	if err := tpl.Execute(&rfCode, filler); err != nil {
		return "", err
	}
	return rfCode.String(), nil
}

// Predict generates a Python program for train a TensorFlow model.
func Predict(ir *ir.PredictStmt, session *pb.Session, modelName, cwd string) (string, error) {
	// FIXME(typhoonzero): if the model not exist on OSS, assume it's a random forest model
	// should use a general method to fetch the model and see the model type.
	exists, err := ossFileExists(modelName)
	if err != nil {
		return "", err
	}
	if !exists {
		log.Printf("predicting using pai random forests")
		return predictRandomForests(ir, session)
	}
	cc, err := GetClusterConfig(ir.Attributes)
	if err != nil {
		return "", err
	}
	program, err := TFLoadAndPredict(ir, session, modelName)
	if err != nil {
		return "", err
	}
	return wrapper(program, session.DbConnStr, modelName, cwd,
		ir.TmpPredictTable, "", ir.ResultTable, cc)
}

// TFLoadAndPredict generates PAI-TF prediction program.
func TFLoadAndPredict(ir *ir.PredictStmt, session *pb.Session, modelName string) (string, error) {
	var tpl = template.Must(template.New("Predict").Parse(tfPredictTmplText))
	ossModelDir, err := FormatCkptDir(modelName)
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

func explainRandomForests(ir *ir.ExplainStmt, session *pb.Session) (string, error) {
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

	filler := &randomForestsExplainFiller{
		DataSource:      session.DbConnStr,
		TmpExplainTable: ir.TmpExplainTable,
		FeatureColumns:  featureFileds,
		LabelColumn:     labelCol.(string),
		Save:            ir.ModelName,
		ResultTable:     ir.Into,
	}
	var tpl = template.Must(template.New("RandomForestsExplain").Parse(randomForestsExplainTemplate))
	var rfCode bytes.Buffer
	if err := tpl.Execute(&rfCode, filler); err != nil {
		return "", err
	}
	return rfCode.String(), nil
}

// Explain generates a Python program for train a TensorFlow model.
func Explain(ir *ir.ExplainStmt, session *pb.Session, modelName, cwd string) (string, error) {
	// NOTE(typhoonzero): only support random forests explain.
	if ir.Into == "" {
		return "", fmt.Errorf("explain PAI random forests model need INTO clause to output the explain result to a table")
	}
	return explainRandomForests(ir, session)
}
