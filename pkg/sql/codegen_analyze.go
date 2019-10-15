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

package sql

import (
	"bytes"
	"fmt"
	"strings"
)

const (
	shapSummaryAttributePrefix = "shap_summary"
)

type analyzeFiller struct {
	*connectionConfig
	X                  []*FeatureMeta
	Label              string
	AnalyzeDatasetSQL  string
	PlotType           string
	ShapSummaryParames map[string]interface{}
	ModelFile          string // path/to/model_file
}

func newAnalyzeFiller(pr *extendedSelect, db *DB, fms []*FeatureMeta, label, modelPath string, summaryAttrs map[string]interface{}) (*analyzeFiller, error) {
	conn, err := newConnectionConfig(db)
	if err != nil {
		return nil, err
	}
	return &analyzeFiller{
		connectionConfig:   conn,
		X:                  fms,
		Label:              label,
		AnalyzeDatasetSQL:  pr.standardSelect.String(),
		ModelFile:          modelPath,
		ShapSummaryParames: summaryAttrs,
	}, nil
}

func readXGBFeatures(pr *extendedSelect, db *DB) ([]*FeatureMeta, string, error) {
	// TODO(weiguo): It's a quick way to read column and label names from
	// xgboost.*, but too heavy.
	fr, err := newXGBFiller(pr, nil, db)
	if err != nil {
		return nil, "", err
	}

	xs := make([]*FeatureMeta, len(fr.X))
	for i := 0; i < len(fr.X); i++ {
		// FIXME(weiguo): we convert xgboost.X to normal(tf).X to reuse
		// DB access API, but I don't think it is a good practice,
		// Think about the AI engines increased, such as ALPS, (EDL?)
		// we should write as many as such converters.
		// How about we unify all featureMetas?
		xs[i] = &FeatureMeta{
			FeatureName: fr.X[i].FeatureName,
			Dtype:       fr.X[i].Dtype,
			Delimiter:   fr.X[i].Delimiter,
			InputShape:  fr.X[i].InputShape,
			IsSparse:    fr.X[i].IsSparse,
		}
	}
	return xs, fr.Y.FeatureName, nil
}

func resolveAnalyzeSummaryParames(atts *attrs) (map[string]interface{}, error) {
	parames, err := resolveAttribute(atts)
	if err != nil {
		return nil, err
	}

	summaryAttrs := make(map[string]interface{})
	for _, v := range parames {
		if v.Prefix == shapSummaryAttributePrefix {
			summaryAttrs[v.Name] = v.Value
		}
	}
	return summaryAttrs, nil
}

func genAnalyzer(pr *extendedSelect, db *DB, cwd, modelDir string) (*bytes.Buffer, error) {
	pr, _, err := loadModelMeta(pr, db, cwd, modelDir, pr.trainedModel)
	if err != nil {
		return nil, fmt.Errorf("loadModelMeta %v", err)
	}
	if !strings.HasPrefix(strings.ToUpper(pr.estimator), `XGBOOST.`) {
		return nil, fmt.Errorf("analyzer: model[%s] not supported", pr.estimator)
	}
	// We untar the XGBoost.{pr.trainedModel}.tar.gz and get three files.
	summaryAttrs, err := resolveAnalyzeSummaryParames(&pr.explainAttrs)
	if err != nil {
		return nil, err
	}

	xs, label, err := readXGBFeatures(pr, db)
	if err != nil {
		return nil, err
	}

	fr, err := newAnalyzeFiller(pr, db, xs, label, pr.trainedModel, summaryAttrs)
	if err != nil {
		return nil, fmt.Errorf("create analyze filler failed: %v", err)
	}

	var program bytes.Buffer
	err = analyzeTemplate.Execute(&program, fr)
	return &program, err
}
