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
	"fmt"
	"strings"

	"sqlflow.org/sqlflow/pkg/sql/codegen"
)

const (
	shapSummaryAttributePrefix = "shap_summary"
)

func GenAnalysis(ir *codegen.AnalyzeIR) (string, error) {
	if !strings.HasPrefix(strings.ToUpper(ir.TrainIR.Estimator), "XGBOOST.") {
		return "", fmt.Errorf("unsupported model:%s", ir.TrainIR.Estimator)
	}
	if ir.Explainer != "TreeExplainer" {
		return "", fmt.Errorf("unsupported explainer")
	}
	summaryAttrs, err := resolveSummaryParames(ir.Attributes)
	if err != nil {
		return "", err
	}

	fr := &filler{
		DataSource:         ir.DataSource,
		DatasetSQL:         ir.Select,
		ShapSummaryParames: summaryAttrs,
		// X []*FeatureMeta
		// Label
		// ModelFile
	}
	var analysis bytes.Buffer
	if err := template.Execute(&analysis, fr); err != nil {
		return "", err
	}
	return analysis.String(), nil
}

// func readXGBFeatures(pr *extendedSelect, db *DB) ([]*FeatureMeta, string, error) {
// 	// TODO(weiguo): It's a quick way to read column and label names from
// 	// xgboost.*, but too heavy.
// 	// NOTE(typhoonzero): analyze does not need to pass session to set hive_location etc.
// 	fr, err := newXGBFiller(pr, nil, db, nil)
// 	if err != nil {
// 		return nil, "", err
// 	}
//
// 	xs := make([]*FeatureMeta, len(fr.X))
// 	for i := 0; i < len(fr.X); i++ {
// 		// FIXME(weiguo): we convert xgboost.X to normal(tf).X to reuse
// 		// DB access API, but I don't think it is a good practice,
// 		// Think about the AI engines increased, such as ALPS, (EDL?)
// 		// we should write as many as such converters.
// 		// How about we unify all featureMetas?
// 		xs[i] = &FeatureMeta{
// 			FeatureName: fr.X[i].FeatureName,
// 			Dtype:       fr.X[i].Dtype,
// 			Delimiter:   fr.X[i].Delimiter,
// 			InputShape:  fr.X[i].InputShape,
// 			IsSparse:    fr.X[i].IsSparse,
// 		}
// 	}
// 	return xs, fr.Y.FeatureName, nil
// }

func resolveSummaryParames(attrs map[string]interface{}) (map[string]interface{}, error) {
	ret := make(map[string]interface{})
	return ret, nil
}

// func resolveAnalyzeSummaryParames(atts *attrs) (map[string]interface{}, error) {
// 	parames, err := resolveAttribute(atts)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	summaryAttrs := make(map[string]interface{})
// 	for _, v := range parames {
// 		if v.Prefix == shapSummaryAttributePrefix {
// 			summaryAttrs[v.Name] = v.Value
// 		}
// 	}
// 	return summaryAttrs, nil
// }
