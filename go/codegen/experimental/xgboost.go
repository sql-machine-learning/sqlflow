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
	"reflect"
	"strings"
	"text/template"

	"sqlflow.org/sqlflow/go/attribute"
	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

type xgbTrainFiller struct {
	StepIndex         int
	DataSource        string
	Select            string
	ValidationSelect  string
	ModelParamsJSON   string
	TrainParamsJSON   string
	FeatureColumnCode string
	LabelColumnCode   string
	DiskCache         bool
	BatchSize         int
	Epoch             int
	Submitter         string
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
	featureColumnCode := ""
	if len(trainStmt.Features) == 1 {
		featureColumnCode, err = generateFeatureColumnCode(trainStmt.Features["feature_columns"])
		if err != nil {
			return "", err
		}
	}
	labelColumnCode, err := generateFeatureColumnCode([]ir.FeatureColumn{trainStmt.Label})

	mp, err := json.Marshal(params[""])
	if err != nil {
		return "", err
	}
	tp, err := json.Marshal(params["train."])
	if err != nil {
		return "", err
	}
	submitter := os.Getenv("SQLFLOW_submitter")
	if submitter == "" {
		submitter = "local"
	}

	filler := xgbTrainFiller{
		StepIndex:         stepIndex,
		DataSource:        session.DbConnStr,
		Select:            strings.Trim(trainStmt.Select, " \n"),
		ValidationSelect:  strings.Trim(trainStmt.ValidationSelect, " \n"),
		ModelParamsJSON:   string(mp),
		TrainParamsJSON:   string(tp),
		FeatureColumnCode: featureColumnCode,
		LabelColumnCode:   labelColumnCode,
		DiskCache:         diskCache,
		BatchSize:         batchSize,
		Epoch:             epoch,
		Submitter:         submitter,
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
    import tempfile
    import os
    import runtime
    import runtime.local
    import runtime.local.xgboost
    import runtime.feature.column as fc
    import runtime.feature.field_desc as fd
    from runtime.model import EstimatorType
    from runtime.xgboost.dataset import xgb_dataset
    import runtime.xgboost as xgboost_extended

    model_params = json.loads('''{{.ModelParamsJSON}}''')
    train_params = json.loads('''{{.TrainParamsJSON}}''')

    ds = "{{.DataSource}}"
    is_pai = False
    pai_train_table = ""
    select = "{{.Select}}"
    val_select = "{{.ValidationSelect}}"
    conn = runtime.db.connect_with_data_source(ds)

    {{ if .FeatureColumnCode }}
    feature_column_map = {"feature_columns": [{{.FeatureColumnCode}}]}
    {{ else }}
    feature_column_map = None
    {{ end }}
    label_fc = {{.LabelColumnCode}}
    label_meta = json.loads(label_fc.get_field_desc()[0].to_json())

    fc_map_ir, fc_label_ir = runtime.feature.infer_feature_columns(conn, select, feature_column_map, label_fc, n=1000)
    fc_map = runtime.feature.compile_ir_feature_columns(fc_map_ir, EstimatorType.XGBOOST)
    feature_column_list = fc_map["feature_columns"]
    feature_metas_obj_list = runtime.feature.get_ordered_field_descs(fc_map_ir)
    feature_metas = dict()
    for fd in feature_metas_obj_list:
        feature_metas[fd.name] = json.loads(fd.to_json())
    feature_column_names = [fd.name for fd in feature_metas_obj_list]

    # NOTE: in the current implementation, we are generating a transform_fn from COLUMN clause. 
    # The transform_fn is executed during the process of dumping the original data into DMatrix SVM file.
    transform_fn = xgboost_extended.feature_column.ComposedColumnTransformer(feature_column_names, *feature_column_list)

    with tempfile.TemporaryDirectory() as tmp_dir_name:
        train_fn = os.path.join(tmp_dir_name, 'train.txt')
        val_fn = os.path.join(tmp_dir_name, 'val.txt')
        dtrain = xgb_dataset(ds, train_fn, select, feature_metas,
                             feature_column_names, label_meta, is_pai,
                             pai_train_table, transform_fn=transform_fn)
        if val_select:
            dval = xgb_dataset(ds, val_fn, val_select, feature_metas,
                               feature_column_names, label_meta, is_pai,
                               pai_train_table, transform_fn=transform_fn)
        else:
            dval = None
        eval_result = runtime.{{.Submitter}}.xgboost.train(dtrain, train_params, model_params, dval)
`

func generateFeatureColumnCode(fcList []ir.FeatureColumn) (string, error) {
	fcCodes := make([]string, 0, len(fcList))
	for _, fc := range fcList {
		// xgboost have no cross feature column, just get the first field desc
		fd := fc.GetFieldDesc()[0]
		// pass format = "" to let runtime feature derivation to fill it in.
		tmpl := `fc.%s(fd.FieldDesc(name="%s", dtype=fd.DataType.%s, delimiter="%s", format="", shape=%s, is_sparse=%s, vocabulary=%s))`
		fcTypeName := reflect.TypeOf(fc).Elem().Name()
		isSparseStr := "False"
		if fd.IsSparse {
			isSparseStr = "True"
		}
		vocabList := []string{}
		for k := range fd.Vocabulary {
			vocabList = append(vocabList, k)
		}
		shape := []int{1}
		if len(fd.Shape) != 0 {
			shape = fd.Shape
		}

		code := fmt.Sprintf(tmpl, fcTypeName, fd.Name,
			strings.ToUpper(ir.DTypeToString(fd.DType)),
			fd.Delimiter,
			ir.AttrToPythonValue(shape),
			isSparseStr,
			ir.AttrToPythonValue(vocabList))
		fcCodes = append(fcCodes, code)
	}

	return strings.Join(fcCodes, ",\n"), nil
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
