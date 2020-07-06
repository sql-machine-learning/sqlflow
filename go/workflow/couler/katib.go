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

package couler

import (
	"encoding/json"
	"fmt"
	"strings"

	"sqlflow.org/sqlflow/go/attribute"
	"sqlflow.org/sqlflow/go/ir"
)

var attributeDictionary = attribute.Dictionary{}.
	Float("eta", float32(0.3), `[default=0.3, alias: learning_rate]
Step size shrinkage used in update to prevents overfitting. After each boosting step, we can directly get the weights of new features, and eta shrinks the feature weights to make the boosting process more conservative.
range: [0,1]`, attribute.Float32RangeChecker(0, 1, true, true)).
	Int("num_class", nil, `Number of classes.
range: [2, Infinity]`, attribute.IntLowerBoundChecker(2, true)).
	String("objective", nil, `Learning objective`, nil).
	IntList("range.num_round", nil, `[ default=[50, 100] ] The range of number of rounds for boosting.`, nil).
	IntList("range.max_depth", nil, `[ default=[2, 8] ] The range of max depth during training.`, nil).
	String("validation.select", nil, `[default=""]
Specify the dataset for validation.
example: "SELECT * FROM boston.train LIMIT 8"`, nil)

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

// ParseKatibSQL generates Couler Katib step
func ParseKatibSQL(t *ir.TrainStmt) (*sqlStatement, error) {
	ss := &sqlStatement{}
	ss.IsKatibTrain = true

	ss.OriginalSQL = t.OriginalSQL

	params, err := parseAttribute(t.Attributes)
	if err != nil {
		return nil, err
	}

	model, booster, err := resolveModelType(t.Estimator)
	if err != nil {
		return nil, err
	}
	params["booster"] = booster
	ss.Model = model

	hps, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	ss.Parameters = string(hps)

	return ss, nil
}
