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
	Columns []string
	Label   string
	// feature names:
	// model: booster
	// model file
	// explainer: TreeExplainer
}

func newAnalyzeFiller(db *DB, columns []string, label string) (*analyzeFiller, error) {
	conn, err := newConnectionConfig(db)
	if err != nil {
		return nil, err
	}
	return &analyzeFiller{
		connectionConfig: conn,
		Columns:          columns,
		Label:            label,
	}, nil
}

func readFeatureNames(pr *extendedSelect, db *DB) ([]string, string, error) {
	if strings.HasPrefix(strings.ToUpper(pr.estimator), `XGBOOST.`) {
		// TODO(weiguo): It's a quick way to read column and label names from
		// xgboost.*, but too heavy.
		xgbFiller, err := newXGBoostFiller(pr, nil, db)
		if err != nil {
			return nil, "", err
		}
		return xgbFiller.FeatureColumns, xgbFiller.Label, nil
	}
	return nil, "", fmt.Errorf("analyzer: model[%s] not supported", pr.estimator)
}

func genAnalyzer(pr *extendedSelect, db *DB, cwd string, modelDir string) (*bytes.Buffer, error) {
	pr, _, err := loadModelMeta(pr, db, cwd, modelDir, pr.trainedModel)
	if err != nil {
		return nil, fmt.Errorf("loadModelMeta %v", err)
	}

	columns, label, err := readFeatureNames(pr, db)
	if err != nil {
		return nil, fmt.Errorf("read feature names err: %v", err)
	}
	fr, err := newAnalyzeFiller(db, columns, label)
	if err != nil {
		return nil, fmt.Errorf("create analyze filler failed: %v", err)
	}

	var program bytes.Buffer
	err = analyzeTemplate.Execute(&program, fr)
	return &program, err
}
