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

package tensorflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"

	"sqlflow.org/sqlflow/go/codegen"

	"sqlflow.org/sqlflow/go/attribute"
	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

var commonAttributes = attribute.Dictionary{}.
	Int("train.batch_size", 1, `[default=1]
The training batch size.
range: [1,Infinity]`, attribute.IntLowerBoundChecker(1, true)).
	Int("train.epoch", 1, `[default=1]
Number of epochs the training will run.
range: [1, Infinity]`, attribute.IntLowerBoundChecker(1, true)).
	Int("train.verbose", 0, `[default=0]
Show verbose logs when training.
possible values: 0, 1, 2`, attribute.IntChoicesChecker(0, 1, 2)).
	Int("train.max_steps", 0, `[default=0]
Max steps to run training.`, attribute.IntLowerBoundChecker(0, true)).
	Int("train.save_checkpoints_steps", 100, `[default=100]
Steps to run between saving checkpoints.`, attribute.IntLowerBoundChecker(1, true)).
	Int("train.log_every_n_iter", 10, `[default=10]
Print logs every n iterations`, attribute.IntLowerBoundChecker(1, true)).
	Int("validation.start_delay_secs", 0, `[default=0]
Seconds to wait before starting validation.`, attribute.IntLowerBoundChecker(0, true)).
	Int("validation.throttle_secs", 0, `[default=0]
Seconds to wait when need to run validation again.`, attribute.IntLowerBoundChecker(0, true)).
	String("validation.metrics", "Accuracy", `[default=""]
Specify metrics when training and evaluating.
example: "Accuracy,AUC"`, nil).
	String("validation.select", "", `[default=""]
Specify the dataset for validation.
example: "SELECT * FROM iris.train LIMIT 100"`, nil).
	Int("validation.steps", 1, `[default=1]
Specify steps for validation.`, attribute.IntLowerBoundChecker(1, true))

var distributedTrainingAttributes = attribute.Dictionary{}.
	Int("train.num_ps", 0, "", nil).
	Int("train.num_workers", 1, "", nil).
	Int("train.worker_cpu", 400, "", nil).
	Int("train.worker_gpu", 0, "", nil).
	Int("train.ps_cpu", 200, "", nil).
	Int("train.ps_gpu", 0, "", nil).
	Int("train.num_evaluator", 0, "", nil).
	Int("train.evaluator_cpu", 200, "", nil).
	Int("train.evaluator_gpu", 0, "", nil)

// TODO(shendiaomo): Make the optimizer related code more general and exported in `attribute.go` if other frameworks
// than TensorFlow have to support python objects as model attributes.

func attrIsOptimizer(attrKey string) bool {
	switch attrKey {
	case "model.optimizer", "model.dnn_optimizer", "model.linear_optimizer":
		return true
	}
	return false
}

// IsPAI tells if we are using PAI platform currently
func IsPAI() bool {
	return os.Getenv("SQLFLOW_submitter") == "pai" || os.Getenv("SQLFLOW_submitter") == "alisa" || os.Getenv("SQLFLOW_submitter") == "pai_local"
}

func setDefaultOptimizer(trainStmt *ir.TrainStmt, optimizerParamName string) {
	// TODO(shendiaomo): Try to get the default value from the python `inspect` module instead of hard coding
	defaultValue := "Adagrad" // Defaults to DNN with a single optimizer parameter
	switch trainStmt.Estimator {
	case "LinearClassifier", "LinearRegressor":
		defaultValue = "Ftrl"
	case "DNNLinearCombinedClassifier", "DNNLinearCombinedRegressor":
		if optimizerParamName == "linear_optimizer" {
			defaultValue = "Ftrl"
		}
	}
	trainStmt.Attributes[optimizerParamName] = defaultValue
}

// constructOptimizers generates a python optimizer object using:
// model.optimizer = "OptimizerName"
// optimizer.arg1 = 1
// optimizer.arg2 = "2"
// To:
// model.optimizer = "OptimizerName(arg1=1, arg2=\"2\")"
func constructOptimizers(trainStmt *ir.TrainStmt) {
	optimizerArgs := map[string]map[string]interface{}{}
	for k, v := range trainStmt.Attributes {
		if attrIsOptimizer(k) {
			if optimizerArgs[k] == nil {
				optimizerArgs[k] = map[string]interface{}{}
			}
		}
		pieces := strings.Split(k, ".")
		if len(pieces) == 2 {
			if attrIsOptimizer("model." + pieces[0]) { // k is like "optimizer.learning_rate"
				if optimizerArgs["model."+pieces[0]] == nil {
					optimizerArgs["model."+pieces[0]] = map[string]interface{}{}
				}
				optimizerArgs["model."+pieces[0]][pieces[1]] = v
				// delete these attributes because they are only used to initialized the python object
				delete(trainStmt.Attributes, k)
			}
		}
	}
	tf1OptimizerClsNames := map[string]string{
		"Adagrad": "tf.train.AdagradOptimizer",
		"Adam":    "tf.train.AdamOptimizer",
		"Ftrl":    "tf.train.FtrlOptimizer",
		"RMSProp": "tf.train.RMSPropOptimizer",
		"SGD":     "tf.train.GradientDescentOptimizer",
	}

	for optimizerParamName, args := range optimizerArgs {
		if _, ok := trainStmt.Attributes[optimizerParamName]; !ok {
			setDefaultOptimizer(trainStmt, optimizerParamName)
		}
		optimizerCls := fmt.Sprintf("%v", trainStmt.Attributes[optimizerParamName])
		if cls, ok := tf1OptimizerClsNames[optimizerCls]; ok && IsPAI() {
			optimizerCls = cls
		}
		optimizerInitPyCode := fmt.Sprintf("%s(", optimizerCls)
		for k, v := range args {
			optimizerInitPyCode += fmt.Sprintf("%s=%v, ", k, v)
		}
		optimizerInitPyCode += ")"
		trainStmt.Attributes[optimizerParamName] = optimizerInitPyCode
	}
}

// constructLosses generate a python loss function call using:
// model.loss = "LossName"
// loss.arg1 = 1
// loss.arg2 = "2"
// To:
// model.loss = "LossName(arg1=1, arg2=\"2\")"
func constructLosses(trainStmt *ir.TrainStmt) {
	lossFunction := ""
	lossArgs := []string{}
	for k, v := range trainStmt.Attributes {
		attrParts := strings.Split(k, ".")
		if k == "model.loss" {
			lossFunction = v.(string)
			continue
		}
		if attrParts[0] == "loss" {
			lossArgs = append(lossArgs, fmt.Sprintf("%s=%v", attrParts[1], v))
			// NOTE(typhoonzero): delete keys in loop is safe:
			// https://stackoverflow.com/questions/23229975/is-it-safe-to-remove-selected-keys-from-map-within-a-range-loop
			delete(trainStmt.Attributes, k)
		}
	}
	if lossFunction != "" {
		lossCode := fmt.Sprintf("%s(%s)", lossFunction, strings.Join(lossArgs, ","))
		trainStmt.Attributes["model.loss"] = lossCode
	}
}

// InitializeAttributes initializes the attributes of TensorFlow and does type checking for them
func InitializeAttributes(trainStmt *ir.TrainStmt) error {
	attribute.ExtractSQLFlowModelsSymbolOnce()
	commonAttributes.ExportDefaults(trainStmt.Attributes)

	modelAttr := attribute.NewDictionaryFromModelDefinition(trainStmt.Estimator, "model.")
	// TODO(shendiaomo): Restrict optimizer parameters to the available set
	constructOptimizers(trainStmt)
	constructLosses(trainStmt)
	if len(modelAttr) == 0 {
		// TODO(shendiaomo): Use the same mechanism as `sqlflow_models` to extract parameters automatically
		// unknownType custom models
		modelAttr.Update(attribute.Dictionary{}.
			Unknown("model.*", nil, "Any model parameters defined in custom models", nil))
	}
	attrValidator := modelAttr.Update(commonAttributes)
	if strings.HasPrefix(trainStmt.Estimator, "sqlflow_models.") {
		// Special attributes defined as global variables in `sqlflow_models`
		modelAttr.Update(attribute.Dictionary{}.
			Unknown("model.optimizer", nil, "Specify optimizer", nil).
			Unknown("model.loss", nil, "Specify loss", nil).
			Unknown("model.*", nil, "Any model parameters defined in custom models", nil))
	}
	if IsPAI() {
		modelAttr.Update(distributedTrainingAttributes)
	}
	return attrValidator.Validate(trainStmt.Attributes)
}

// CategorizeAttributes returns attributes like train.*, validation.* and model.*  to separated maps.
func CategorizeAttributes(trainStmt *ir.TrainStmt) (trainParams, validateParams, modelParams map[string]interface{}) {
	trainParams = make(map[string]interface{})
	validateParams = make(map[string]interface{})
	modelParams = make(map[string]interface{})

	for attrKey, attr := range trainStmt.Attributes {
		if strings.HasPrefix(attrKey, "train.") {
			trainParams[strings.Replace(attrKey, "train.", "", 1)] = attr
		}
		if strings.HasPrefix(attrKey, "model.") {
			modelParams[strings.Replace(attrKey, "model.", "", 1)] = attr
		}
		if strings.HasPrefix(attrKey, "validation.") {
			validateParams[strings.Replace(attrKey, "validation.", "", 1)] = attr
		}
	}
	return trainParams, validateParams, modelParams
}

// DeriveFeatureColumnCodeAndFieldDescs generates tensorflow feature column code and field descs from IR.
func DeriveFeatureColumnCodeAndFieldDescs(trainStmt *ir.TrainStmt) (featureColumnsCode []string, fieldDescs map[string][]*ir.FieldDesc, err error) {
	fieldDescs = make(map[string][]*ir.FieldDesc)
	for target, fcList := range trainStmt.Features {
		perTargetFeatureColumnsCode := []string{}
		for _, fc := range fcList {
			fcCode, err := codegen.GenerateFeatureColumnCode(fc, "tf")
			if err != nil {
				return nil, nil, err
			}
			perTargetFeatureColumnsCode = append(perTargetFeatureColumnsCode, fcCode)
			if len(fc.GetFieldDesc()) > 0 {
				for _, fm := range fc.GetFieldDesc() {
					_, ok := fieldDescs[target]
					if !ok {
						fieldDescs[target] = []*ir.FieldDesc{}
					}
					fieldDescs[target] = append(fieldDescs[target], fm)
				}
			}
		}
		featureColumnsCode = append(featureColumnsCode,
			fmt.Sprintf("\"%s\": [%s]", target, strings.Join(perTargetFeatureColumnsCode, ",\n")))
	}
	return featureColumnsCode, fieldDescs, nil
}

// Train generates a Python program for train a TensorFlow model.
func Train(trainStmt *ir.TrainStmt, session *pb.Session) (string, error) {
	trainParams, validateParams, modelParams := CategorizeAttributes(trainStmt)
	featureColumnsCode, fieldDescs, err := DeriveFeatureColumnCodeAndFieldDescs(trainStmt)
	if err != nil {
		return "", err
	}

	// Need to create tmp table for train/validate when using PAI
	paiTrainTable := ""
	paiValidateTable := ""
	if IsPAI() && trainStmt.TmpTrainTable != "" {
		paiTrainTable = trainStmt.TmpTrainTable
		paiValidateTable = trainStmt.TmpValidateTable
	}

	filler := trainFiller{
		DataSource:          session.DbConnStr,
		TrainSelect:         trainStmt.Select,
		ValidationSelect:    trainStmt.ValidationSelect,
		Estimator:           trainStmt.Estimator,
		FieldDescs:          fieldDescs,
		FeatureColumnCode:   fmt.Sprintf("{%s}", strings.Join(featureColumnsCode, ",\n")),
		Y:                   trainStmt.Label.GetFieldDesc()[0], // TODO(typhoonzero): label only support numericColumn.
		ModelParams:         modelParams,
		TrainParams:         trainParams,
		ValidationParams:    validateParams,
		Save:                "model_save",
		LoadPreTrainedModel: trainStmt.PreTrainedModel != "",
		IsPAI:               IsPAI(),
		PAITrainTable:       paiTrainTable,
		PAIValidateTable:    paiValidateTable,
		ModelRepoImage:      trainStmt.ModelImage,
		OriginalSQL:         trainStmt.OriginalSQL,
	}
	var program bytes.Buffer
	var trainTemplate = template.Must(template.New("Train").Funcs(template.FuncMap{
		"intArrayToJSONString": ir.MarshalToJSONString,
		"attrToPythonValue":    ir.AttrToPythonValue,
		"DTypeToString":        ir.DTypeToString,
	}).Parse(tfTrainTemplateText))
	if err := trainTemplate.Execute(&program, filler); err != nil {
		return "", err
	}
	return program.String(), nil
}

// Pred generates a Python program for predict using a TensorFlow model.
func Pred(predStmt *ir.PredictStmt, session *pb.Session) (string, error) {
	modelParams, featureColumnsCode, fieldDescs, err := restoreModel(predStmt.TrainStmt)
	if err != nil {
		return "", err
	}
	trainLabelFM := predStmt.TrainStmt.Label.GetFieldDesc()[0]
	predLabelFM := &ir.FieldDesc{
		Name:       predStmt.ResultColumn,
		DType:      trainLabelFM.DType,
		Delimiter:  trainLabelFM.Delimiter,
		Format:     trainLabelFM.Format,
		Shape:      trainLabelFM.Shape,
		IsSparse:   trainLabelFM.IsSparse,
		Vocabulary: trainLabelFM.Vocabulary,
		MaxID:      trainLabelFM.MaxID,
	}

	filler := predFiller{
		DataSource:        session.DbConnStr,
		Select:            predStmt.Select,
		ResultTable:       predStmt.ResultTable,
		Estimator:         predStmt.TrainStmt.Estimator,
		FieldDescs:        fieldDescs,
		FeatureColumnCode: fmt.Sprintf("{%s}", strings.Join(featureColumnsCode, ",\n")),
		TrainLabelMeta:    trainLabelFM,
		PredLabelMeta:     predLabelFM,
		ModelParams:       modelParams,
		Save:              "model_save",
		HDFSNameNodeAddr:  session.HdfsNamenodeAddr,
		HiveLocation:      session.HiveLocation,
		HDFSUser:          session.HdfsUser,
		HDFSPass:          session.HdfsPass,
	}
	var program bytes.Buffer
	var predTemplate = template.Must(template.New("Pred").Funcs(template.FuncMap{
		"intArrayToJSONString": ir.MarshalToJSONString,
		"attrToPythonValue":    ir.AttrToPythonValue,
		"DTypeToString":        ir.DTypeToString,
	}).Parse(tfPredTemplateText))
	if err := predTemplate.Execute(&program, filler); err != nil {
		return "", err
	}

	return program.String(), nil
}

// Explain generates a Python program to explain a trained model.
func Explain(stmt *ir.ExplainStmt, session *pb.Session) (string, error) {
	modelParams, featureColumnsCode, fieldDescs, err := restoreModel(stmt.TrainStmt)
	if err != nil {
		return "", err
	}
	labelFM := stmt.TrainStmt.Label.GetFieldDesc()[0]

	const summaryAttrPrefix = "summary."
	summaryAttrs := resolveParams(stmt.Attributes, summaryAttrPrefix)
	jsonSummary, err := json.Marshal(summaryAttrs)
	if err != nil {
		return "", err
	}

	filler := explainFiller{
		DataSource:        session.DbConnStr,
		Select:            stmt.Select,
		SummaryParams:     string(jsonSummary),
		EstimatorClass:    stmt.TrainStmt.Estimator,
		FieldDescs:        fieldDescs,
		FeatureColumnCode: fmt.Sprintf("{%s}", strings.Join(featureColumnsCode, ",\n")),
		Y:                 labelFM,
		ModelParams:       modelParams,
		Save:              "model_save",
		ResultTable:       stmt.Into,
		HDFSNameNodeAddr:  session.HdfsNamenodeAddr,
		HiveLocation:      session.HiveLocation,
		HDFSUser:          session.HdfsUser,
		HDFSPass:          session.HdfsPass,
	}
	var program bytes.Buffer
	var tmpl = template.Must(template.New("Explain").Funcs(template.FuncMap{
		"intArrayToJSONString": ir.MarshalToJSONString,
		"attrToPythonValue":    ir.AttrToPythonValue,
		"DTypeToString":        ir.DTypeToString,
	}).Parse(boostedTreesExplainTemplateText))
	if err := tmpl.Execute(&program, filler); err != nil {
		return "", err
	}
	return program.String(), nil
}

// Evaluate generates a Python program to evaluate a trained model.
func Evaluate(stmt *ir.EvaluateStmt, session *pb.Session) (string, error) {
	modelParams, featureColumnsCode, fieldDescs, err := restoreModel(stmt.TrainStmt)
	if err != nil {
		return "", err
	}
	labelFM := stmt.TrainStmt.Label.GetFieldDesc()[0]
	validationParams := resolveParams(stmt.Attributes, "validation.")
	if len(validationParams) == 0 {
		// add default validation.metrics = "Accuracy".
		validationParams["metrics"] = "Accuracy"
	}

	filler := evaluateFiller{
		DataSource:        session.DbConnStr,
		Select:            stmt.Select,
		Estimator:         stmt.TrainStmt.Estimator,
		FieldDescs:        fieldDescs,
		FeatureColumnCode: fmt.Sprintf("{%s}", strings.Join(featureColumnsCode, ",\n")),
		Y:                 labelFM,
		ModelParams:       modelParams,
		ValidationParams:  validationParams,
		Save:              "model_save",
		ResultTable:       stmt.Into,
		HDFSNameNodeAddr:  session.HdfsNamenodeAddr,
		HiveLocation:      session.HiveLocation,
		HDFSUser:          session.HdfsUser,
		HDFSPass:          session.HdfsPass,
	}
	var program bytes.Buffer
	var tmpl = template.Must(template.New("Evaluate").Funcs(template.FuncMap{
		"intArrayToJSONString": ir.MarshalToJSONString,
		"attrToPythonValue":    ir.AttrToPythonValue,
		"DTypeToString":        ir.DTypeToString,
	}).Parse(tfEvaluateTemplateText))
	if err := tmpl.Execute(&program, filler); err != nil {
		return "", err
	}
	return program.String(), nil
}

// restoreModel reconstruct necessary python objects from TrainStmt
func restoreModel(stmt *ir.TrainStmt) (modelParams map[string]interface{}, featureColumnsCode []string, fieldDescs map[string][]*ir.FieldDesc, err error) {
	fieldDescs = make(map[string][]*ir.FieldDesc)
	modelParams = make(map[string]interface{})
	for attrKey, attr := range stmt.Attributes {
		if strings.HasPrefix(attrKey, "model.") {
			modelParams[strings.Replace(attrKey, "model.", "", 1)] = attr
		}
	}
	featureColumnsCode, fieldDescs, err = DeriveFeatureColumnCodeAndFieldDescs(stmt)
	return
}

// make a exported function in outer package
func resolveParams(attrs map[string]interface{}, group string) map[string]interface{} {
	sp := make(map[string]interface{})
	for k, v := range attrs {
		if strings.HasPrefix(k, group) {
			sp[k[len(group):]] = v
		}
	}
	return sp
}
