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

	"sqlflow.org/sqlflow/pkg/sql/codegen/attribute"

	"sqlflow.org/sqlflow/pkg/sql/codegen"
)

func newFloat32(f float32) *float32 {
	return &f
}

func newInt(i int) *int {
	return &i
}

// TODO(tony): complete model parameter and training parameter list
// model parameter list: https://xgboost.readthedocs.io/en/latest/parameter.html#general-parameters
// training parameter list: https://github.com/dmlc/xgboost/blob/b61d53447203ca7a321d72f6bdd3f553a3aa06c4/python-package/xgboost/training.py#L115-L117
var attributeDictionary = attribute.Dictionary{
	"eta": {attribute.Float, `[default=0.3, alias: learning_rate]
Step size shrinkage used in update to prevents overfitting. After each boosting step, we can directly get the weights of new features, and eta shrinks the feature weights to make the boosting process more conservative.
range: [0,1]`, attribute.Float32RangeChecker(newFloat32(0), newFloat32(1), true, true)},
	"num_class": {attribute.Int, `Number of classes.
range: [1, Infinity]`, attribute.IntRangeChecker(newInt(0), nil, false, false)},
	"objective": {attribute.String, `Learning objective`, nil},
	"train.num_boost_round": {attribute.Int, `[default=10]
The number of rounds for boosting.
range: [1, Infinity]`, attribute.IntRangeChecker(newInt(0), nil, false, false)},
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
		return "", fmt.Errorf("unsupport model name %v, currently supports xgboost.gbtree, xgboost.gblinear, xgboost.dart", estimator)
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

func getFieldMeta(fcs []codegen.FeatureColumn, l codegen.FeatureColumn) ([]codegen.FieldMeta, codegen.FieldMeta, error) {
	var features []codegen.FieldMeta
	for _, fc := range fcs {
		switch c := fc.(type) {
		case *codegen.NumericColumn:
			features = append(features, *c.FieldMeta)
		default:
			return nil, codegen.FieldMeta{}, fmt.Errorf("unsupported feature column type %T on %v", c, c)
		}
	}

	var label codegen.FieldMeta
	switch c := l.(type) {
	case *codegen.NumericColumn:
		label = *c.FieldMeta
	default:
		return nil, codegen.FieldMeta{}, fmt.Errorf("unsupported label column type %T on %v", c, c)
	}

	return features, label, nil
}

// Train generates a Python program for train a XgBoost model.
func Train(ir *codegen.TrainIR) (string, error) {
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
func Pred(ir *codegen.PredictIR) (string, error) {
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
		DataSource:      ir.DataSource,
		PredSelect:      ir.Select,
		FeatureMetaJSON: string(f),
		LabelMetaJSON:   string(l),
	}

	var program bytes.Buffer

	if err := predTemplate.Execute(&program, r); err != nil {
		return "", nil
	}
	return program.String(), nil
}
