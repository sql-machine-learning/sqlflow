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
// limitations under the License.o

package xgboost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"sqlflow.org/sqlflow/pkg/sql/ir"
)

const (
	shapSummaryAttrPrefix = "shap_summary."
)

// Explain generates a Python program to explain a trained model.
func Explain(analyzeStmt *ir.AnalyzeStmt) (string, error) {
	if analyzeStmt.Explainer != "TreeExplainer" {
		return "", fmt.Errorf("unsupported explainer %s", analyzeStmt.Explainer)
	}
	summaryParams := resolveParams(analyzeStmt.Attributes, shapSummaryAttrPrefix)
	jsonSummary, err := json.Marshal(summaryParams)
	if err != nil {
		return "", err
	}
	xs, y, err := getFieldDesc(analyzeStmt.TrainStmt.Features["feature_columns"], analyzeStmt.TrainStmt.Label)
	if err != nil {
		return "", err
	}
	fm, err := json.Marshal(xs)
	if err != nil {
		return "", err
	}

	fr := &analyzeFiller{
		DataSource:           analyzeStmt.DataSource,
		DatasetSQL:           analyzeStmt.Select,
		ShapSummaryParams:    string(jsonSummary),
		FeatureFieldMetaJSON: string(fm),
		LabelName:            y.Name,
	}
	var analysis bytes.Buffer
	if err := analyzeTemplate.Execute(&analysis, fr); err != nil {
		return "", err
	}
	return analysis.String(), nil
}

func resolveParams(attrs map[string]interface{}, group string) map[string]interface{} {
	sp := make(map[string]interface{})
	for k, v := range attrs {
		if strings.HasPrefix(k, group) {
			sp[k[len(group):]] = v
		}
	}
	return sp
}
