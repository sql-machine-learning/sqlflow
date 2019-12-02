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
	"strings"

	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen/attribute"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

// TODO(tony): complete model parameter and training parameter list
// model parameter list: https://xgboost.readthedocs.io/en/latest/parameter.html#general-parameters
// training parameter list: https://github.com/dmlc/xgboost/blob/b61d53447203ca7a321d72f6bdd3f553a3aa06c4/python-package/xgboost/training.py#L115-L117
var attributeDictionary = attribute.Dictionary{
	"eta": {attribute.Float, `[default=0.3, alias: learning_rate]
Step size shrinkage used in update to prevents overfitting. After each boosting step, we can directly get the weights of new features, and eta shrinks the feature weights to make the boosting process more conservative.
range: [0,1]`, attribute.Float32RangeChecker(0, 1, true, true)},
	"num_class": {attribute.Int, `Number of classes.
range: [2, Infinity]`, attribute.IntLowerBoundChecker(2, true)},
	"objective": {attribute.String, `Learning objective`, nil},
	"train.num_boost_round": {attribute.Int, `[default=10]
The number of rounds for boosting.
range: [1, Infinity]`, attribute.IntLowerBoundChecker(1, true)},
	"validation.select": {attribute.String, `[default=""]
Specify the dataset for validation.
example: "SELECT * FROM boston.train LIMIT 8"`, nil},
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

func parseAttribute(attrs map[string]interface{}) (map[string]map[string]interface{}, error) {
	if err := attributeDictionary.Validate(attrs); err != nil {
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

func getFieldMeta(fcs []ir.FeatureColumn, l ir.FeatureColumn) ([]ir.FieldMeta, ir.FieldMeta, error) {
	var features []ir.FieldMeta
	for _, fc := range fcs {
		switch c := fc.(type) {
		case *ir.NumericColumn:
			features = append(features, *c.FieldMeta)
		default:
			return nil, ir.FieldMeta{}, fmt.Errorf("unsupported feature column type %T on %v", c, c)
		}
	}

	var label ir.FieldMeta
	switch c := l.(type) {
	case *ir.NumericColumn:
		label = *c.FieldMeta
	default:
		return nil, ir.FieldMeta{}, fmt.Errorf("unsupported label column type %T on %v", c, c)
	}

	return features, label, nil
}

// Train generates a Python program for train a XgBoost model.
func Train(ir *ir.TrainClause) (string, error) {
	params, err := parseAttribute(ir.Attributes)
	if err != nil {
		return "", err
	}
	booster, err := resolveModelType(ir.Estimator)
	if err != nil {
		return "", err
	}
	params[""]["booster"] = booster

	if len(ir.Features) != 1 {
		return "", fmt.Errorf("xgboost only support 1 feature column set, received %d", len(ir.Features))
	}
	featureFieldMeta, labelFieldMeta, err := getFieldMeta(ir.Features["feature_columns"], ir.Label)
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
	f, err := json.Marshal(featureFieldMeta)
	if err != nil {
		return "", err
	}
	l, err := json.Marshal(labelFieldMeta)
	if err != nil {
		return "", err
	}
	r := trainFiller{
		DataSource:       ir.DataSource,
		TrainSelect:      ir.Select,
		ValidationSelect: ir.ValidationSelect,
		ModelParamsJSON:  string(mp),
		TrainParamsJSON:  string(tp),
		FieldMetaJSON:    string(f),
		LabelJSON:        string(l)}

	var program bytes.Buffer
	if err := trainTemplate.Execute(&program, r); err != nil {
		return "", err
	}

	return program.String(), nil
}

// Pred generates a Python program for predict a xgboost model.
func Pred(ir *ir.PredictClause, session *pb.Session) (string, error) {
	featureFieldMeta, labelFieldMeta, err := getFieldMeta(ir.TrainIR.Features["feature_columns"], ir.TrainIR.Label)
	f, err := json.Marshal(featureFieldMeta)
	if err != nil {
		return "", err
	}
	l, err := json.Marshal(labelFieldMeta)
	if err != nil {
		return "", err
	}

	r := predFiller{
		DataSource:       ir.DataSource,
		PredSelect:       ir.Select,
		FeatureMetaJSON:  string(f),
		LabelMetaJSON:    string(l),
		ResultTable:      ir.ResultTable,
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
