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

package sql

import (
	"fmt"
	"io"
	"strings"
)

// Code generator for XGBoost
type XGBGenerator struct{
	*commonFiller
	estimatorType string
}

func (*XGBGenerator) execute(w io.Writer) error {
	return fmt.Errorf("XGBGenerator has not been implemented")
}

func newXGBGenerator(r *commonFiller) (*XGBGenerator, error) {
	var xgbEstimatorType string
	switch strings.ToUpper(r.Estimator) {
	case "XGBOOSTCLASSIFIER":
		xgbEstimatorType = "XGBoostClassifier"
	case "XGBOOSTREGRESSOR":
		xgbEstimatorType = "XGBoostRegressor"
	case "XGBOOSTESTIMATOR":
		xgbEstimatorType = "XGBoostEstimator"
	default:
		xgbEstimatorType = ""
	}
	if len(xgbEstimatorType) == 0 {
		return nil, nil
	}
	temp := &XGBGenerator{r, xgbEstimatorType}
	return temp, nil
}
