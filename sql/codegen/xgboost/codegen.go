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
	"github.com/sql-machine-learning/sqlflow/sql/codegen"
	"strings"
)

var attributeChecker = map[string]func(interface{}) error{
	"eta": func(x interface{}) error {
		switch x.(type) {
		case float32, float64:
			return nil
		default:
			return fmt.Errorf("eta should be of type float, received %T", x)
		}
	},
	"num_class": func(x interface{}) error {
		switch x.(type) {
		case int, int32, int64:
			return nil
		default:
			return fmt.Errorf("num_class should be of type int, received %T", x)
		}
	},
	"train.num_boost_round": func(x interface{}) error {
		switch x.(type) {
		case int, int32, int64:
			return nil
		default:
			return fmt.Errorf("train.num_boost_round should be of type int, received %T", x)
		}
	},
	"objective": func(x interface{}) error {
		if _, ok := x.(string); !ok {
			return fmt.Errorf("objective should be of type string, received %T", x)
		}
		return nil
	},
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

func parseAttribute(attrs []codegen.Attribute) (map[string]map[string]interface{}, error) {
	attrNames := map[string]bool{}

	params := map[string]map[string]interface{}{"": {}, "train.": {}}
	for _, attr := range attrs {
		if _, ok := attrNames[attr.Key]; ok {
			return nil, fmt.Errorf("duplicated attribute %s", attr.Key)
		}
		attrNames[attr.Key] = true
		checker, ok := attributeChecker[attr.Key]
		if !ok {
			return nil, fmt.Errorf("unrecognized attribute %v", attr.Key)
		}
		if err := checker(attr.Value); err != nil {
			return nil, err
		}
		for prefix, paramMap := range params {
			if strings.HasPrefix(attr.Key, prefix) {
				paramMap[attr.Key[len(prefix):]] = attr.Value
			}
		}
	}

	return params, nil
}

func getFieldMeta(fcs []codegen.FeatureColumn, l codegen.FeatureColumn) ([]codegen.FieldMeta, codegen.FieldMeta, error) {
	var features []codegen.FieldMeta
	for _, fc := range fcs {
		switch c := fc.(type) {
		case codegen.NumericColumn:
			features = append(features, *c.FieldMeta)
		default:
			return nil, codegen.FieldMeta{}, fmt.Errorf("unsupported feature column type %T on %v", c, c)
		}
	}

	var label codegen.FieldMeta
	switch c := l.(type) {
	case codegen.NumericColumn:
		label = *c.FieldMeta
	default:
		return nil, codegen.FieldMeta{}, fmt.Errorf("unsupported label column type %T on %v", c, c)
	}

	return features, label, nil
}

// Train generates a Python program for train a XgBoost model.
func Train(ir codegen.TrainIR) (string, error) {
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
