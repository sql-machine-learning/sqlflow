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

package couler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen/attribute"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

var defaultKatibDockerImage = "sqlflow/sqlflow"

var attributeDictionary = attribute.Dictionary{
	"eta": {attribute.Float, `[default=0.3, alias: learning_rate]
Step size shrinkage used in update to prevents overfitting. After each boosting step, we can directly get the weights of new features, and eta shrinks the feature weights to make the boosting process more conservative.
range: [0,1]`, attribute.Float32RangeChecker(0, 1, true, true)},
	"num_class": {attribute.Int, `Number of classes.
range: [2, Infinity]`, attribute.IntLowerBoundChecker(2, true)},
	"objective":       {attribute.String, `Learning objective`, nil},
	"range.num_round": {attribute.IntList, `[ default=[50, 100] ] The range of number of rounds for boosting.`, nil},
	"range.max_depth": {attribute.IntList, `[ default=[2, 8] ] The range of max depth during training.`, nil},
	"validation.select": {attribute.String, `[default=""]
Specify the dataset for validation.
example: "SELECT * FROM boston.train LIMIT 8"`, nil},
}

func resolveModelType(estimator string) (string, string, error) {
	switch strings.ToUpper(estimator) {
	case "XGBOOST.GBTREE":
		return "xgboost", "gbtree", nil
	case "XGBOOST.GBLINEAR":
		return "xgboost", "gblinear", nil
	case "XGBOOST.DART":
		return "xgboost", "dart", nil
	default:
		return "", "", fmt.Errorf("unsupported model name %v, currently supports xgboost.gbtree, xgboost.gblinear, xgboost.dart", estimator)
	}
}

func parseAttribute(attrs map[string]interface{}) (map[string]interface{}, error) {
	if err := attributeDictionary.Validate(attrs); err != nil {
		return nil, err
	}

	params := map[string]map[string]interface{}{"": {}, "range.": {}}
	paramPrefix := []string{"range.", ""} // use slice to assure traverse order, this is necessary because all string starts with ""
	for key, attr := range attrs {
		for _, pp := range paramPrefix {
			if strings.HasPrefix(key, pp) {
				params[pp][key[len(pp):]] = attr
			}
		}
	}

	return params["range."], nil
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

// RunKatib generates Couler Katib program
func RunKatib(t ir.TrainStmt, session *pb.Session) (string, error) {
	ss := &coulerKatibFiller{}
	ss.IsExtendedSQL = true

	ss.Select = t.Select
	ss.Validation = t.ValidationSelect
	ss.OriginalSQL = t.OriginalSQL

	params, err := parseAttribute(t.Attributes)
	if err != nil {
		return "", err
	}

	model, booster, err := resolveModelType(t.Estimator)
	if err != nil {
		return "", err
	}
	params["booster"] = booster

	hps, err := json.Marshal(params)
	if err != nil {
		return "", err
	}
	ss.ModelParamsJSON = string(hps)

	featureFieldDesc, labelFieldDesc, err := getFieldDesc(t.Features["feature_columns"], t.Label)
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

	ss.FieldDescJSON = string(f)
	ss.LabelJSON = string(l)
	ss.Model = model

	var program bytes.Buffer
	if err := coulerKatibTemplate.Execute(&program, ss); err != nil {
		return "", err
	}
	fmt.Printf(program.String())
	return program.String(), nil
}
