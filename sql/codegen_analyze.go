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

type analyzeFiller struct {
	*connectionConfig
	X                 []*featureMeta
	Label             string
	AnalyzeDatasetSQL string
	PlotType          string
	ModelFile         string // path/to/model_file
}

func newAnalyzeFiller(pr *extendedSelect, db *DB, fms []*featureMeta, label, modelPath, plotType string) (*analyzeFiller, error) {
	conn, err := newConnectionConfig(db)
	if err != nil {
		return nil, err
	}
	return &analyzeFiller{
		connectionConfig: conn,
		X:                fms,
		Label:            label,
		// TODO(weiguo): test if it needs TrimSuffix(SQL, ";") on hive,
		// or we trim it in pr(*extendedSelect)
		AnalyzeDatasetSQL: pr.standardSelect.String(),
		ModelFile:         modelPath,
		PlotType:          plotType,
	}, nil
}

func readAntXGBFeatures(pr *extendedSelect, db *DB) ([]*featureMeta, string, error) {
	// TODO(weiguo): It's a quick way to read column and label names from
	// xgboost.*, but too heavy.
	fr, err := newAntXGBoostFiller(pr, nil, db)
	if err != nil {
		return nil, "", err
	}

	xs := make([]*featureMeta, len(fr.X))
	for i := 0; i < len(fr.X); i++ {
		// FIXME(weiguo): we convert xgboost.X to normal(tf).X to reuse
		// DB access API, but I don't think it is a good practice,
		// Think about the AI engines increased, such as ALPS, (EDL?)
		// we should write as many as such converters.
		// How about we unify all featureMetas?
		xs[i] = &featureMeta{
			FeatureName: fr.X[i].FeatureName,
			Dtype:       fr.X[i].Dtype,
			Delimiter:   fr.X[i].Delimiter,
			InputShape:  fr.X[i].InputShape,
			IsSparse:    fr.X[i].IsSparse,
		}
	}
	return xs, fr.Label, nil
}

func readPlotType(pr *extendedSelect) string {
	v, ok := pr.analyzeAttrs["shap.plot_type"]
	if !ok {
		// using shap default value
		return `""`
	}
	return v.val
}

func genAnalyzer(pr *extendedSelect, db *DB, cwd, modelDir string) (*bytes.Buffer, error) {
	pr, _, err := loadModelMeta(pr, db, cwd, modelDir, pr.trainedModel)
	if err != nil {
		return nil, fmt.Errorf("loadModelMeta %v", err)
	}
	if !strings.HasPrefix(strings.ToUpper(pr.estimator), `XGBOOST.`) {
		return nil, fmt.Errorf("analyzer: model[%s] not supported", pr.estimator)
	}
	// We untar the AntXGBoost.{pr.trainedModel}.tar.gz and get three files.
	// Here, the sqlflow_booster is a raw xgboost binary file can be analyzed.
	antXGBModelPath := fmt.Sprintf("%s/sqlflow_booster", pr.trainedModel)
	plotType := readPlotType(pr)
	xs, label, err := readAntXGBFeatures(pr, db)
	if err != nil {
		return nil, err
	}

	fr, err := newAnalyzeFiller(pr, db, xs, label, antXGBModelPath, plotType)
	if err != nil {
		return nil, fmt.Errorf("create analyze filler failed: %v", err)
	}

	var program bytes.Buffer
	err = analyzeTemplate.Execute(&program, fr)
	return &program, err
}
