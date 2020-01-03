// Copyright 2019 The SQLFlow Authors. All rights reserved.
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

	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen/attribute"
	"sqlflow.org/sqlflow/pkg/sql/ir"
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
	"objective": {attribute.String, nil, `Learning objective`, nil},
	"train.num_boost_round": {attribute.Int, 10, `[default=10]
The number of rounds for boosting.
range: [1, Infinity]`, attribute.IntLowerBoundChecker(1, true)},
	"validation.select": {attribute.String, "", `[default=""]
Specify the dataset for validation.
example: "SELECT * FROM boston.train LIMIT 8"`, nil},
}
var fullAttrValidator = attribute.Dictionary{}

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

func parseAttribute(attrs map[string]interface{}) (map[string]map[string]interface{}, error) {
	attributeDictionary.FillDefaults(attrs)
	if err := fullAttrValidator.Validate(attrs); err != nil {
		return nil, err
	}

	params := map[string]map[string]interface{}{"": {}, "train.": {}}
	paramPrefix := []string{"train.", ""} // use slice to assure traverse order, this is necessary because all string starts with ""
	for key, attr := range attrs {
		for _, pp := range paramPrefix {
			if strings.HasPrefix(key, pp) {
				params[pp][key[len(pp):]] = attr
			}
		}
	}

	return params, nil
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

// Train generates a Python program for train a XgBoost model.
func Train(trainStmt *ir.TrainStmt, session *pb.Session) (string, error) {
	params, err := parseAttribute(trainStmt.Attributes)
	if err != nil {
		return "", err
	}
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
	f, err := json.Marshal(featureFieldDesc)
	if err != nil {
		return "", err
	}
	l, err := json.Marshal(labelFieldDesc)
	if err != nil {
		return "", err
	}
	r := trainFiller{
		DataSource:       session.DbConnStr,
		TrainSelect:      trainStmt.Select,
		ValidationSelect: trainStmt.ValidationSelect,
		ModelParamsJSON:  string(mp),
		TrainParamsJSON:  string(tp),
		FieldDescJSON:    string(f),
		LabelJSON:        string(l)}

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
	f, err := json.Marshal(featureFieldDesc)
	if err != nil {
		return "", err
	}
	l, err := json.Marshal(labelFieldDesc)
	if err != nil {
		return "", err
	}

	r := predFiller{
		DataSource:       session.DbConnStr,
		PredSelect:       predStmt.Select,
		FeatureMetaJSON:  string(f),
		LabelMetaJSON:    string(l),
		ResultTable:      predStmt.ResultTable,
		HDFSNameNodeAddr: session.HdfsNamenodeAddr,
		HiveLocation:     session.HiveLocation,
		HDFSUser:         session.HdfsUser,
		HDFSPass:         session.HdfsPass,
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
