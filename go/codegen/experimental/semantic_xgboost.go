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
	"fmt"
	"strings"

	"sqlflow.org/sqlflow/go/attribute"
	"sqlflow.org/sqlflow/go/ir"
)

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

func resolveXGBoostModelParams(ir *ir.TrainStmt) error {
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

func init() {
	// xgboost.gbtree, xgboost.dart, xgboost.gblinear share the same parameter set
	fullAttrValidator = attribute.NewDictionaryFromModelDefinition("xgboost.gbtree", "")
	fullAttrValidator.Update(attributeDictionary)
}

// -----------------------------------------------------------------------------
