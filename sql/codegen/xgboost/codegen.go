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
	"github.com/sql-machine-learning/sqlflow/sql"
	"strings"
)

var attributeChecker = map[string]func(interface{}) error{
	"model.eta": func(x interface{}) error {
		_, ok1 := x.(float32)
		_, ok2 := x.(float64)
		if !ok1 && !ok2 {
			return fmt.Errorf("model.eta should be of type float, received %T", x)
		}
		return nil
	},
	"model.num_class": func(x interface{}) error {
		if _, ok := x.(int); !ok {
			return fmt.Errorf("model.num_class should be of type int, received %T", x)
		}
		return nil
	},
	"train.num_boost_round": func(x interface{}) error {
		if _, ok := x.(int); !ok {
			return fmt.Errorf("train.num_boost_round should be of type int, received %T", x)
		}
		return nil
	},
	"model.objective": func(x interface{}) error {
		if _, ok := x.(string); !ok {
			return fmt.Errorf("model.objective should be of type string, received %T", x)
		}
		return nil
	},
}

func parseAttribute(attrs map[string]interface{}) (map[string]map[string]interface{}, error) {
	params := map[string]map[string]interface{}{"model.": {}, "train.": {}}
	for k, v := range attrs {
		checker, ok := attributeChecker[k]
		if !ok {
			return nil, fmt.Errorf("unrecognized attribute %v", k)
		}
		if err := checker(v); err != nil {
			return nil, err
		}
		for prefix, paramMap := range params {
			if strings.HasPrefix(k, prefix) {
				paramMap[k[len(prefix):]] = v
			}
		}
	}

	return params, nil
}

// Train generates a Python program for train a XgBoost model.
func Train(ir sql.TrainIR) (string, error) {
	params, err := parseAttribute(ir.Attribute)
	if err != nil {
		return "", err
	}
	if len(ir.Feature) != 1 {
		return "", fmt.Errorf("xgboost only support 1 feature column set, received %d", len(ir.Feature))
	}

	mp, err := json.Marshal(params["model."])
	if err != nil {
		return "", err
	}
	tp, err := json.Marshal(params["train."])
	if err != nil {
		return "", err
	}
	f, err := json.Marshal(ir.Feature["feature_columns"])
	if err != nil {
		return "", err
	}
	l, err := json.Marshal(ir.Label)
	if err != nil {
		return "", err
	}
	r := trainFiller{
		DataSource:       ir.DataSource,
		TrainSelect:      ir.Select,
		ValidationSelect: ir.ValidationSelect,
		ModelParamsJSON:  string(mp),
		TrainParamsJSON:  string(tp),
		FeatureJSON:      string(f),
		LabelJSON:        string(l)}

	var program bytes.Buffer
	if err := trainTemplate.Execute(&program, r); err != nil {
		return "", err
	}

	return program.String(), nil
}
