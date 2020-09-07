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
// limitations under the License.o

package xgboost

import (
	"bytes"
	"encoding/json"
	"strings"

	tf "sqlflow.org/sqlflow/go/codegen/tensorflow"
	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

const (
	shapSummaryAttrPrefix = "summary."
)

// Explain generates a Python program to explain a trained model.
func Explain(explainStmt *ir.ExplainStmt, session *pb.Session) (string, error) {
	summaryParams := resolveParams(explainStmt.Attributes, shapSummaryAttrPrefix)
	jsonSummary, err := json.Marshal(summaryParams)
	if err != nil {
		return "", err
	}

	featureColumnCode, xs, y, err := deriveFeatureColumnCodeAndFieldDescs(explainStmt.TrainStmt.Features["feature_columns"], explainStmt.TrainStmt.Label)
	if err != nil {
		return "", err
	}
	f, fs, err := resolveFeatureMeta(xs)
	if err != nil {
		return "", err
	}

	l, err := json.Marshal(resolveFieldMeta(&y))
	if err != nil {
		return "", err
	}

	fr := &explainFiller{
		DataSource:           session.DbConnStr,
		DatasetSQL:           explainStmt.Select,
		ShapSummaryParams:    string(jsonSummary),
		Explainer:            explainStmt.Explainer,
		FeatureFieldMetaJSON: string(f),
		FeatureColumnNames:   fs,
		FeatureColumnCode:    featureColumnCode,
		LabelJSON:            string(l),
		ResultTable:          explainStmt.Into,
		IsPAI:                tf.IsPAI(),
		PAIExplainTable:      explainStmt.TmpExplainTable,
	}
	var analysis bytes.Buffer
	if err := explainTemplate.Execute(&analysis, fr); err != nil {
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
