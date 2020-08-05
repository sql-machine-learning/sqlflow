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
	"sqlflow.org/sqlflow/go/codegen"
	tf "sqlflow.org/sqlflow/go/codegen/tensorflow"
	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

type xgbTrainFiller struct {
	DataSource         string
	Select             string
	ValidationSelect   string
	ModelParamsJSON    string
	TrainParamsJSON    string
	FieldDescJSON      string
	LabelJSON          string
	FeatureColumnNames []string
	FeatureColumnCode  string
	DiskCache          bool
	BatchSize          int
	Epoch              int
	Submitter          string
}

// XGBoostGenerateTrain returns the step code
func XGBoostGenerateTrain(trainStmt *ir.TrainStmt, session *pb.Session) (string, error) {
	if err := resolveModelParams(trainStmt); err != nil {
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

	// TODO(typhoonzero): use feature derivation at runtime.
	if len(trainStmt.Features) != 1 {
		return "", fmt.Errorf("xgboost only support 0 or 1 feature column set, received %d", len(trainStmt.Features))
	}

	featureColumnCode, featureFieldDesc, labelFieldDesc, err := deriveFeatureColumnCodeAndFieldDescs(trainStmt.Features["feature_columns"], trainStmt.Label)
	if err != nil {
		return "", err
	}
	mp, err := json.Marshal(params[""])
	if err != nil {
		return "", err
	}
	tp, err := json.Marshal(params["train."])
	if err != nil {
		return "", err
	}
	f, fs, err := resolveFeatureMeta(featureFieldDesc)
	if err != nil {
		return "", err
	}
	l, err := json.Marshal(resolveFieldMeta(&labelFieldDesc))
	if err != nil {
		return "", err
	}

	filler := xgbTrainFiller{
		DataSource:         session.DbConnStr,
		Select:             trainStmt.Select,
		ValidationSelect:   trainStmt.ValidationSelect,
		ModelParamsJSON:    string(mp),
		TrainParamsJSON:    string(tp),
		FieldDescJSON:      string(f),
		LabelJSON:          string(l),
		FeatureColumnNames: fs,
		FeatureColumnCode:  featureColumnCode,
		DiskCache:          diskCache,
		BatchSize:          batchSize,
		Epoch:              epoch,
		Submitter:          os.Getenv("SQLFLOW_submitter"),
	}
	var program bytes.Buffer
	var trainTemplate = template.Must(template.New("Train").Parse(xgbTrainTemplate))
	err = trainTemplate.Execute(&program, filler)
	if err != nil {
		return "", err
	}
	return program.String(), nil
}

var xgbTrainTemplate = `
def step_entry():
    import json
    import runtime
    from runtime.xgboost.dataset import xgb_dataset

    model_params = json.loads('''{{.ModelParamsJSON}}''')
    train_params = json.loads('''{{.TrainParamsJSON}}''')
    feature_metas = json.loads('''{{.FieldDescJSON}}''')
    label_meta = json.loads('''{{.LabelJSON}}''')

    ds = "{{.DataSource}}"
    is_pai = False
    pai_train_table = ""
    select = "{{.Select}}"
    val_select = "{{.ValidationSelect}}"

    # Derive feature columns at runtime like:
    # fcmap, fc_label = infer_feature_columns(conn, select, features, label, n=1000)

    feature_column_names = [{{range .FeatureColumnNames}}
    "{{.}}",
    {{end}}]

    # NOTE: in the current implementation, we are generating a transform_fn from COLUMN clause. 
    # The transform_fn is executed during the process of dumping the original data into DMatrix SVM file.
    feature_column_list = [{{.FeatureColumnCode}}]
    transform_fn = xgboost_extended.feature_column.ComposedColumnTransformer(feature_column_names, *feature_column_list)

    with tempfile.TemporaryDirectory() as tmp_dir_name:
        train_fn = os.path.join(tmp_dir_name, 'train.txt')
        val_fn = os.path.join(tmp_dir_name, 'val.txt')
        dtrain = xgb_dataset(ds, train_fn, select, feature_metas,
                             feature_column_names, label_meta, is_pai,
                             pai_train_table, transform_fn=transform_fn)
        dval = xgb_dataset(ds, val_fn, val_select, feature_metas,
                           feature_column_names, label_meta, is_pai,
                           pai_train_table, transform_fn=transform_fn)
        eval_result = runtime.{{.Submitter}}.xgboost.train(dtrain, train_params, model_params, dval)
`

// TODO(typhoonzero): below functions are copied from codegen/xgboost/codegen.go
// remove the original functions when this experimental packages are ready.

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

// FieldMeta delicates Field Meta with Json format which used in code generator
type FieldMeta struct {
	FeatureName string `json:"feature_name"`
	DType       string `json:"dtype"`
	Delimiter   string `json:"delimiter"`
	Format      string `json:"format"`
	Shap        []int  `json:"shape"`
	IsSparse    bool   `json:"is_sparse"`
}

func resolveFieldMeta(desc *ir.FieldDesc) FieldMeta {
	return FieldMeta{
		FeatureName: desc.Name,
		DType:       tf.DTypeToString(desc.DType),
		Delimiter:   desc.Delimiter,
		Format:      desc.Format,
		Shap:        desc.Shape,
		IsSparse:    desc.IsSparse,
	}
}

func resolveFeatureMeta(fds []ir.FieldDesc) ([]byte, []string, error) {
	ret := make(map[string]FieldMeta)
	featureNames := []string{}
	for _, f := range fds {
		ret[f.Name] = resolveFieldMeta(&f)
		featureNames = append(featureNames, f.Name)
	}
	f, e := json.Marshal(ret)
	return f, featureNames, e
}

// deriveFeatureColumnCodeAndFieldDescs generates the feature column codes and feature descs, which are used for
// codegen in Python codes.
// The returned feature column code is like "xgboost_extended.feature_column.numeric(...)".
// The returned feature descs contain all field descs used in feature column code.
func deriveFeatureColumnCodeAndFieldDescs(fcs []ir.FeatureColumn, labelFc ir.FeatureColumn) (featureColumnsCode string, fieldDescs []ir.FieldDesc, label ir.FieldDesc, err error) {
	if fcs == nil {
		return "", nil, ir.FieldDesc{}, fmt.Errorf("feature_columns should not be nil")
	}

	fcCodes := make([]string, 0, len(fcs))
	for _, fc := range fcs {
		code, err := codegen.GenerateFeatureColumnCode(fc, "xgboost_extended")
		if err != nil {
			return "", nil, ir.FieldDesc{}, err
		}

		fcCodes = append(fcCodes, code)

		for _, desc := range fc.GetFieldDesc() {
			fieldDescs = append(fieldDescs, *desc)
		}
	}

	featureColumnsCode = strings.Join(fcCodes, ",\n")

	switch c := labelFc.(type) {
	case *ir.NumericColumn:
		label = *c.FieldDesc
	default:
		return "", nil, ir.FieldDesc{}, fmt.Errorf("unsupported label column type %T on %v", c, c)
	}

	return featureColumnsCode, fieldDescs, label, err
}

func init() {
	// xgboost.gbtree, xgboost.dart, xgboost.gblinear share the same parameter set
	fullAttrValidator = attribute.NewDictionaryFromModelDefinition("xgboost.gbtree", "")
	fullAttrValidator.Update(attributeDictionary)
}
