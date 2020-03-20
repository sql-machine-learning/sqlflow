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

package xgboost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"sqlflow.org/sqlflow/pkg/ir"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen/attribute"
	tf "sqlflow.org/sqlflow/pkg/sql/codegen/tensorflow"
)

// TODO(tony): complete model parameter and training parameter list
// model parameter list: https://xgboost.readthedocs.io/en/latest/parameter.html#general-parameters
// training parameter list: https://github.com/dmlc/xgboost/blob/b61d53447203ca7a321d72f6bdd3f553a3aa06c4/python-package/xgboost/training.py#L115-L117
var attributeDictionary = attribute.Dictionary{
	"eta": {attribute.Float, float32(0.3), `[default=0.3, alias: learning_rate]
Step size shrinkage used in update to prevents overfitting. After each boosting step, we can directly get the weights of new features, and eta shrinks the feature weights to make the boosting process more conservative.
range: [0,1]`, attribute.Float32RangeChecker(0, 1, true, true)},
	"num_class": {attribute.Int, nil, `Number of classes.
range: [2, Infinity]`, attribute.IntLowerBoundChecker(2, true)},
	"objective":   {attribute.String, nil, `Learning objective`, objectiveChecker},
	"eval_metric": {attribute.String, nil, `eval metric`, nil},
	"train.num_boost_round": {attribute.Int, 10, `[default=10]
The number of rounds for boosting.
range: [1, Infinity]`, attribute.IntLowerBoundChecker(1, true)},
	"validation.select": {attribute.String, "", `[default=""]
Specify the dataset for validation.
example: "SELECT * FROM boston.train LIMIT 8"`, nil},
}
var fullAttrValidator = attribute.Dictionary{}

func objectiveChecker(obj interface{}) error {
	s, ok := obj.(string)
	if !ok {
		return fmt.Errorf("expected type string, received %T", obj)
	}
	expected := []string{
		"reg:squarederror",
		"reg:squaredlogerror",
		"reg:logistic",
		"binary:logistic",
		"binary:logitraw",
		"binary:hinge",
		"survival:cox",
		"multi:softmax",
		"multi:softprob",
		"rank:pairwise",
		"rank:ndcg",
		"rank:map",
		"reg:gamma",
		"reg:tweedie"}
	for _, e := range expected {
		if s == e {
			return nil
		}
	}
	return fmt.Errorf("unrecognized objective %s, should be one of %v", s, expected)
}

func resolveModelType(estimator string) (string, error) {
	switch strings.ToUpper(estimator) {
	case "XGBOOST.GBTREE":
		return "gbtree", nil
	case "XGBOOST.GBLINEAR":
		return "gblinear", nil
	case "XGBOOST.DART":
		return "dart", nil
	default:
		return "", fmt.Errorf("unsupported model name %v, currently supports xgboost.gbtree, xgboost.gblinear, xgboost.dart", estimator)
	}
}

// InitializeAttributes initializes the attributes of XGBoost and does type checking for them
func InitializeAttributes(trainStmt *ir.TrainStmt) error {
	attributeDictionary.FillDefaults(trainStmt.Attributes)
	return fullAttrValidator.Validate(trainStmt.Attributes)
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

func getFieldDesc(fcs []ir.FeatureColumn, l ir.FeatureColumn) ([]ir.FieldDesc, ir.FieldDesc, error) {
	var features []ir.FieldDesc
	for _, fc := range fcs {
		switch c := fc.(type) {
		case *ir.NumericColumn:
			features = append(features, *c.FieldDesc)
		default:
			return nil, ir.FieldDesc{}, fmt.Errorf("unsupported feature column type %T on %v", c, c)
		}
	}

	var label ir.FieldDesc
	switch c := l.(type) {
	case *ir.NumericColumn:
		label = *c.FieldDesc
	default:
		return nil, ir.FieldDesc{}, fmt.Errorf("unsupported label column type %T on %v", c, c)
	}

	return features, label, nil
}

// FieldMeta delicates Field Meta with Json format which used in code generator
type FieldMeta struct {
	FeatureName string `json:"feature_name"`
	DType       string `json:"dtype"`
	Delimiter   string `json:"delimiter"`
	Shap        []int  `json:"shape"`
	IsSparse    bool   `json:"is_sparse"`
}

func resolveFieldMeta(desc *ir.FieldDesc) FieldMeta {
	return FieldMeta{
		FeatureName: desc.Name,
		DType:       tf.DTypeToString(desc.DType),
		Delimiter:   desc.Delimiter,
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

// Train generates a Python program for train a XgBoost model.
func Train(trainStmt *ir.TrainStmt, session *pb.Session) (string, error) {
	params := parseAttribute(trainStmt.Attributes)
	booster, err := resolveModelType(trainStmt.Estimator)
	if err != nil {
		return "", err
	}
	params[""]["booster"] = booster

	if len(trainStmt.Features) != 1 {
		return "", fmt.Errorf("xgboost only support 1 feature column set, received %d", len(trainStmt.Features))
	}
	featureFieldDesc, labelFieldDesc, err := getFieldDesc(trainStmt.Features["feature_columns"], trainStmt.Label)
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

	paiTrainTable := ""
	paiValidateTable := ""
	if tf.IsPAI() && trainStmt.TmpTrainTable != "" {
		paiTrainTable = trainStmt.TmpTrainTable
		paiValidateTable = trainStmt.TmpValidateTable
	}

	r := trainFiller{
		DataSource:         session.DbConnStr,
		TrainSelect:        trainStmt.Select,
		ValidationSelect:   trainStmt.ValidationSelect,
		ModelParamsJSON:    string(mp),
		TrainParamsJSON:    string(tp),
		FieldDescJSON:      string(f),
		FeatureColumnNames: fs,
		LabelJSON:          string(l),
		IsPAI:              tf.IsPAI(),
		PAITrainTable:      paiTrainTable,
		PAIValidateTable:   paiValidateTable}

	var program bytes.Buffer
	if err := trainTemplate.Execute(&program, r); err != nil {
		return "", err
	}

	return program.String(), nil
}

// Pred generates a Python program for predict a xgboost model.
func Pred(predStmt *ir.PredictStmt, session *pb.Session) (string, error) {
	featureFieldDesc, labelFieldDesc, err := getFieldDesc(predStmt.TrainStmt.Features["feature_columns"], predStmt.TrainStmt.Label)
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

	paiPredictTable := ""
	if tf.IsPAI() && predStmt.TmpPredictTable != "" {
		paiPredictTable = predStmt.TmpPredictTable
	}

	r := predFiller{
		DataSource:         session.DbConnStr,
		PredSelect:         predStmt.Select,
		FeatureMetaJSON:    string(f),
		FeatureColumnNames: fs,
		LabelMetaJSON:      string(l),
		ResultTable:        predStmt.ResultTable,
		HDFSNameNodeAddr:   session.HdfsNamenodeAddr,
		HiveLocation:       session.HiveLocation,
		HDFSUser:           session.HdfsUser,
		HDFSPass:           session.HdfsPass,
		IsPAI:              tf.IsPAI(),
		PAITable:           paiPredictTable,
	}

	var program bytes.Buffer

	if err := predTemplate.Execute(&program, r); err != nil {
		return "", err
	}
	return program.String(), nil
}

func init() {
	re := regexp.MustCompile("[^a-z]")
	// xgboost.gbtree, xgboost.dart, xgboost.gblinear share the same parameter set
	fullAttrValidator = attribute.NewDictionaryFromModelDefinition("xgboost.gbtree", "")
	for _, v := range fullAttrValidator {
		pieces := strings.SplitN(v.Doc, " ", 2)
		maybeType := re.ReplaceAllString(pieces[0], "")
		if maybeType == strings.ToLower(maybeType) {
			switch maybeType {
			case "float":
				v.Type = attribute.Float
			case "int":
				v.Type = attribute.Int
			case "string":
				v.Type = attribute.String
			}
			v.Doc = pieces[1]
		}
	}
	fullAttrValidator.Update(attributeDictionary)
}
