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

package experimental

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"

	"sqlflow.org/sqlflow/go/attribute"
	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

type xgbTrainFiller struct {
	StepIndex         int
	OriginalSQL       string
	ModelImage        string
	Estimator         string
	DataSource        string
	Select            string
	ValidationSelect  string
	ModelParamsJSON   string
	TrainParamsJSON   string
	FeatureColumnCode string
	LabelColumnCode   string
	Save              string
	Load              string
	DiskCache         bool
	BatchSize         int
	Epoch             int
	Submitter         string
}

func replaceNewLineRuneAndTrimSpace(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

// XGBoostGenerateTrain returns the step code.
func XGBoostGenerateTrain(trainStmt *ir.TrainStmt, stepIndex int, session *pb.Session) (string, error) {
	var err error
	if err = resolveModelParams(trainStmt); err != nil {
		return "", err
	}
	params := parseAttribute(trainStmt.Attributes)
	diskCache := params["train."]["disk_cache"].(bool)
	delete(params["train."], "disk_cache")

	var batchSize, epoch = -1, 1
	batchSizeAttr, ok := params["train."]["batch_size"]
	if ok {
		batchSize = batchSizeAttr.(int)
		delete(params["train."], "batch_size")
	}
	epochAttr, ok := params["train."]["epoch"]
	if ok {
		epoch = epochAttr.(int)
		delete(params["train."], "epoch")
	}
	if _, ok := params["train."]["num_workers"]; ok {
		delete(params["train."], "num_workers")
	}

	if len(trainStmt.Features) > 1 {
		return "", fmt.Errorf("xgboost only support 0 or 1 feature column set, received %d", len(trainStmt.Features))
	}
	// featureColumnCode is a python map definition code like fc_map = {"feature_columns": [...]}
	featureColumnCode := generateFeatureColumnCode(trainStmt.Features)
	labelColumnCode := trainStmt.Label.GenPythonCode()

	mp, err := json.Marshal(params[""])
	if err != nil {
		return "", err
	}
	tp, err := json.Marshal(params["train."])
	if err != nil {
		return "", err
	}

	dbConnStr, err := GeneratePyDbConnStr(session)
	if err != nil {
		return "", err
	}

	filler := xgbTrainFiller{
		StepIndex:         stepIndex,
		OriginalSQL:       replaceNewLineRuneAndTrimSpace(trainStmt.OriginalSQL),
		ModelImage:        trainStmt.ModelImage,
		Estimator:         trainStmt.Estimator,
		DataSource:        dbConnStr,
		Select:            replaceNewLineRuneAndTrimSpace(trainStmt.Select),
		ValidationSelect:  replaceNewLineRuneAndTrimSpace(trainStmt.ValidationSelect),
		ModelParamsJSON:   string(mp),
		TrainParamsJSON:   string(tp),
		FeatureColumnCode: featureColumnCode,
		LabelColumnCode:   labelColumnCode,
		Save:              trainStmt.Into,
		Load:              trainStmt.PreTrainedModel,
		DiskCache:         diskCache,
		BatchSize:         batchSize,
		Epoch:             epoch,
		Submitter:         getSubmitter(session),
	}
	var program bytes.Buffer
	var trainTemplate = template.Must(template.New("Train").Parse(xgbTrainTemplate))
	err = trainTemplate.Execute(&program, filler)
	if err != nil {
		return "", err
	}
	return program.String(), nil
}

const xgbTrainTemplate = `
def step_entry_{{.StepIndex}}():
    import json
    import runtime.temp_file as temp_file
    import runtime.feature.column
    import runtime.feature.field_desc
    from runtime.{{.Submitter}} import train

    feature_column_map = {{.FeatureColumnCode}}
    label_column = {{.LabelColumnCode}}

    model_params = json.loads('''{{.ModelParamsJSON}}''')
    train_params = json.loads('''{{.TrainParamsJSON}}''')

    with temp_file.TemporaryDirectory(as_cwd=True) as temp_dir:
        train_params["original_sql"] = '''{{.OriginalSQL}}'''
        train_params["model_image"] = '''{{.ModelImage}}'''
        train_params["feature_column_map"] = feature_column_map
        train_params["label_column"] = label_column
        train_params["disk_cache"] = "{{.DiskCache}}"=="true"
        train_params["batch_size"] = {{.BatchSize}}
        train_params["epoch"] = {{.Epoch}}

        train(datasource='''{{.DataSource}}''',
              estimator_string='''{{.Estimator}}''',
              select='''{{.Select}}''',
              validation_select='''{{.ValidationSelect}}''',
              model_params=model_params,
              save='''{{.Save}}''',
              load='''{{.Load}}''',
              train_params=train_params)
`

type xgbPredFiller struct {
	StepIndex     int
	DataSource    string
	Select        string
	PredLabelName string
	ResultTable   string
	Load          string
	Submitter     string
}

// XGBoostGeneratePredict generates the XGBoost prediction code
func XGBoostGeneratePredict(predStmt *ir.PredictStmt, stepIndex int, session *pb.Session) (string, error) {
	dbConnStr, err := GeneratePyDbConnStr(session)
	if err != nil {
		return "", err
	}

	filler := &xgbPredFiller{
		StepIndex:     stepIndex,
		DataSource:    dbConnStr,
		Select:        replaceNewLineRuneAndTrimSpace(predStmt.Select),
		PredLabelName: predStmt.ResultColumn,
		ResultTable:   predStmt.ResultTable,
		Load:          predStmt.Using,
		Submitter:     getSubmitter(session),
	}

	var program bytes.Buffer
	predTmpl := template.Must(template.New("Train").Parse(xgbPredTemplate))
	err = predTmpl.Execute(&program, filler)
	if err != nil {
		return "", err
	}
	return program.String(), nil
}

const xgbPredTemplate = `
def step_entry_{{.StepIndex}}():
    import runtime.temp_file as temp_file
    from runtime.{{.Submitter}} import pred
    
    with temp_file.TemporaryDirectory(as_cwd=True):
        pred(datasource='''{{.DataSource}}''', 
             select='''{{.Select}}''', 
             result_table='''{{.ResultTable}}''', 
             pred_label_name='''{{.PredLabelName}}''', 
             load='''{{.Load}}''')
`

type xgbEvaluateFiller struct {
	StepIndex         int
	DataSource        string
	Select            string
	ResultTable       string
	PredLabelName     string
	Load              string
	ValidationMetrics string
	Submitter         string
}

// XGBoostGenerateEvaluation generates the XGBoost evaluation code
func XGBoostGenerateEvaluation(evalStmt *ir.EvaluateStmt, stepIndex int, session *pb.Session) (string, error) {
	ds, err := GeneratePyDbConnStr(session)
	if err != nil {
		return "", err
	}

	labelName := ""
	if nc, ok := evalStmt.Label.(*ir.NumericColumn); ok {
		labelName = nc.FieldDesc.Name
	} else {
		return "", fmt.Errorf("unsupported label type %T", evalStmt.Label)
	}

	metricList := []string{"accuracy_score"}
	if m, ok := evalStmt.Attributes["validation.metrics"]; ok {
		if metricStr, ok := m.(string); ok {
			metricList = []string{}
			for _, s := range strings.Split(metricStr, ",") {
				metricList = append(metricList, strings.TrimSpace(s))
			}
		} else {
			return "", fmt.Errorf("validation.metrics must be of type string")
		}
	}
	metricPyStr := ir.AttrToPythonValue(metricList)

	filler := &xgbEvaluateFiller{
		StepIndex:         stepIndex,
		DataSource:        ds,
		Select:            replaceNewLineRuneAndTrimSpace(evalStmt.Select),
		ResultTable:       evalStmt.Into,
		PredLabelName:     labelName,
		Load:              evalStmt.ModelName,
		ValidationMetrics: metricPyStr,
		Submitter:         getSubmitter(session),
	}

	var program bytes.Buffer
	tpl := template.Must(template.New("Evaluate").Parse(xgbEvaluateTemplate))
	if err := tpl.Execute(&program, filler); err != nil {
		return "", err
	}
	return program.String(), nil
}

const xgbEvaluateTemplate = `
def step_entry_{{.StepIndex}}():
    import runtime.temp_file as temp_file
    from runtime.{{.Submitter}} import evaluate
    
    with temp_file.TemporaryDirectory(as_cwd=True):
        evaluate(datasource='''{{.DataSource}}''', 
                 select='''{{.Select}}''', 
                 result_table='''{{.ResultTable}}''', 
                 pred_label_name='''{{.PredLabelName}}''', 
                 load='''{{.Load}}''',
                 validation_metrics={{.ValidationMetrics}})
`

func getSubmitter(session *pb.Session) string {
	if session.Submitter != "" {
		return session.Submitter
	}

	submitter := os.Getenv("SQLFLOW_submitter")
	if submitter != "" {
		return submitter
	}
	return "local"
}

func generateFeatureColumnCode(fcMap map[string][]ir.FeatureColumn) string {
	allFCCodes := make([]string, 0)
	for target, fcList := range fcMap {
		if len(fcList) == 0 {
			continue
		}
		codeList := make([]string, 0)
		for _, fc := range fcList {
			codeList = append(codeList, fc.GenPythonCode())
		}
		code := fmt.Sprintf(`"%s":[%s]`, target, strings.Join(codeList, ","))
		allFCCodes = append(allFCCodes, code)
	}
	return fmt.Sprintf("{%s}", strings.Join(allFCCodes, ","))
}

// TODO(typhoonzero): below functions are copied from codegen/xgboost/codegen.go
// remove the original functions when this experimental packages are ready.
// -----------------------------------------------------------------------------

func getXGBoostObjectives() (ret []string) {
	for k := range attribute.XGBoostObjectiveDocs {
		ret = append(ret, k)
	}
	return
}

// TODO(tony): complete model parameter and training parameter list
// model parameter list: https://xgboost.readthedocs.io/en/latest/parameter.html#general-parameters
// training parameter list: https://github.com/dmlc/xgboost/blob/b61d53447203ca7a321d72f6bdd3f553a3aa06c4/python-package/xgboost/training.py#L115-L117
var attributeDictionary = attribute.Dictionary{}.
	Float("eta", float32(0.3), `[default=0.3, alias: learning_rate]
Step size shrinkage used in update to prevents overfitting. After each boosting step, we can directly get the weights of new features, and eta shrinks the feature weights to make the boosting process more conservative.
range: [0,1]`, attribute.Float32RangeChecker(0, 1, true, true)).
	Int("num_class", nil, `Number of classes.
range: [2, Infinity]`, attribute.IntLowerBoundChecker(2, true)).
	String("objective", nil, `Learning objective`, attribute.StringChoicesChecker(getXGBoostObjectives()...)).
	String("eval_metric", nil, `eval metric`, nil).
	Bool("train.disk_cache", false, `whether use external memory to cache train data`, nil).
	Int("train.num_boost_round", 10, `[default=10]
The number of rounds for boosting.
range: [1, Infinity]`, attribute.IntLowerBoundChecker(1, true)).
	Int("train.batch_size", -1, `[default=-1]
Batch size for each iteration, -1 means use all data at once.
range: [-1, Infinity]`, attribute.IntLowerBoundChecker(-1, true)).
	Int("train.epoch", 1, `[default=1]
Number of rounds to run the training.
range: [1, Infinity]`, attribute.IntLowerBoundChecker(1, true)).
	String("validation.select", "", `[default=""]
Specify the dataset for validation.
example: "SELECT * FROM boston.train LIMIT 8"`, nil).
	Int("train.num_workers", 1, `[default=1]
Number of workers for distributed train, 1 means stand-alone mode.
range: [1, 128]`, attribute.IntRangeChecker(1, 128, true, true))

var fullAttrValidator = attribute.Dictionary{}

func updateIfKeyDoesNotExist(current, add map[string]interface{}) {
	for k, v := range add {
		if _, ok := current[k]; !ok {
			current[k] = v
		}
	}
}

func resolveModelParams(ir *ir.TrainStmt) error {
	switch strings.ToUpper(ir.Estimator) {
	case "XGBOOST.XGBREGRESSOR", "XGBREGRESSOR":
		defaultAttributes := map[string]interface{}{"objective": "reg:squarederror"}
		updateIfKeyDoesNotExist(ir.Attributes, defaultAttributes)
	case "XGBOOST.XGBRFREGRESSOR", "XGBRFREGRESSOR":
		defaultAttributes := map[string]interface{}{"objective": "reg:squarederror", "learning_rate": 1, "subsample": 0.8, "colsample_bynode": 0.8, "reg_lambda": 1e-05}
		updateIfKeyDoesNotExist(ir.Attributes, defaultAttributes)
	case "XGBOOST.XGBCLASSIFIER", "XGBCLASSIFIER":
		defaultAttributes := map[string]interface{}{"objective": "binary:logistic"}
		updateIfKeyDoesNotExist(ir.Attributes, defaultAttributes)
	case "XGBOOST.XGBRFCLASSIFIER", "XGBRFCLASSIFIER":
		defaultAttributes := map[string]interface{}{"objective": "multi:softprob", "learning_rate": 1, "subsample": 0.8, "colsample_bynode": 0.8, "reg_lambda": 1e-05}
		updateIfKeyDoesNotExist(ir.Attributes, defaultAttributes)
	case "XGBOOST.XGBRANKER", "XGBRANKER":
		defaultAttributes := map[string]interface{}{"objective": "rank:pairwise"}
		updateIfKeyDoesNotExist(ir.Attributes, defaultAttributes)
	case "XGBOOST.GBTREE":
		defaultAttributes := map[string]interface{}{"booster": "gbtree"}
		updateIfKeyDoesNotExist(ir.Attributes, defaultAttributes)
	case "XGBOOST.GBLINEAR":
		defaultAttributes := map[string]interface{}{"booster": "gblinear"}
		updateIfKeyDoesNotExist(ir.Attributes, defaultAttributes)
	case "XGBOOST.DART":
		defaultAttributes := map[string]interface{}{"booster": "dart"}
		updateIfKeyDoesNotExist(ir.Attributes, defaultAttributes)
	default:
		return fmt.Errorf("unsupported model name %v, currently supports xgboost.gbtree, xgboost.gblinear, xgboost.dart", ir.Estimator)
	}
	return nil
}

func parseAttribute(attrs map[string]interface{}) map[string]map[string]interface{} {
	params := map[string]map[string]interface{}{"": {}, "train.": {}}
	paramPrefix := []string{"train.", ""} // use slice to assure traverse order, this is necessary because all string starts with ""
	for key, attr := range attrs {
		for _, pp := range paramPrefix {
			if strings.HasPrefix(key, pp) {
				params[pp][key[len(pp):]] = attr
				break
			}
		}
	}
	return params
}

func init() {
	// xgboost.gbtree, xgboost.dart, xgboost.gblinear share the same parameter set
	fullAttrValidator = attribute.NewDictionaryFromModelDefinition("xgboost.gbtree", "")
	fullAttrValidator.Update(attributeDictionary)
}

// -----------------------------------------------------------------------------
