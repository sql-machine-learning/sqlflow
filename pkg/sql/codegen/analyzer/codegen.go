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

package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"sqlflow.org/sqlflow/pkg/sql/codegen"
)

const (
	shapSummaryAttributes = "shap_summary"
)

// GenAnalysis generates a Python program to analyze a trained model.
func GenAnalysis(ir *codegen.AnalyzeIR, modelPath string) (string, error) {
	if strings.HasPrefix(strings.ToUpper(ir.TrainIR.Estimator), "XGBOOST.") {
		return genXGBAnalysis(ir, modelPath)
	}
	return "", fmt.Errorf("unsupported model:%s", ir.TrainIR.Estimator)
}

func genXGBAnalysis(ir *codegen.AnalyzeIR, modelPath string) (string, error) {
	if ir.Explainer != "TreeExplainer" {
		return "", fmt.Errorf("unsupported explainer")
	}
	summaryAttrs, err := resolveParames(ir.Attributes, shapSummaryAttributes)
	if err != nil {
		return "", err
	}
	xs, y, err := getFieldMeta(ir.TrainIR)
	if err != nil {
		return "", err
	}
	fm, err := json.Marshal(xs)
	if err != nil {
		return "", err
	}

	fr := &filler{
		DataSource:         ir.DataSource,
		DatasetSQL:         ir.Select,
		ShapSummaryParames: summaryAttrs,
		FieldMetaJSON:      string(fm),
		Label:              y.Name,
		ModelFile:          modelPath,
	}
	var analysis bytes.Buffer
	if err := templ.Execute(&analysis, fr); err != nil {
		return "", err
	}
	return analysis.String(), nil
}

func resolveParames(attrs map[string]interface{}, group string) (map[string]interface{}, error) {
	sp := make(map[string]interface{})
	for k, v := range attrs {
		if strings.HasPrefix(k, group) {
			sp[k[len(group):]] = v
		}
	}
	return sp, nil
}

func getFieldMeta(ir *codegen.TrainIR) ([]codegen.FieldMeta, codegen.FieldMeta, error) {
	var features []codegen.FieldMeta
	for _, fc := range ir.Features["feature_columns"] {
		switch c := fc.(type) {
		case *codegen.NumericColumn:
			features = append(features, *c.FieldMeta)
		default:
			return nil, codegen.FieldMeta{}, fmt.Errorf("unsupported feature column type %T on %v", c, c)
		}
	}

	var label codegen.FieldMeta
	switch c := ir.Label.(type) {
	case *codegen.NumericColumn:
		label = *c.FieldMeta
	default:
		return nil, codegen.FieldMeta{}, fmt.Errorf("unsupported label column type %T on %v", c, c)
	}
	return features, label, nil
}
