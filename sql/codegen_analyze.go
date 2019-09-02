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
	"os"
	"strings"
)

type analyzeFiller struct {
	*connectionConfig
	X                 []*featureMeta
	Label             string
	AnalyzeDatasetSQL string
}

func newAnalyzeFiller(pr *extendedSelect, db *DB, fms []*featureMeta, label string) (*analyzeFiller, error) {
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
	}, nil
}

func readFeatureNames(pr *extendedSelect, db *DB) ([]*featureMeta, string, error) {
	if strings.HasPrefix(strings.ToUpper(pr.estimator), `XGBOOST.`) {
		// TODO(weiguo): It's a quick way to read column and label names from
		// xgboost.*, but too heavy.
		fr, err := newAntXGBoostFiller(pr, nil, db)
		if err != nil {
			return nil, "", err
		}

		fms := make([]*featureMeta, len(fr.X))
		for i := 0; i < len(fr.X); i++ {
			// FIXME(weiguo): we convert xgboost.X to normal(tf).X to reuse
			// DB access API, but I don't think it is a good practice,
			// Think about the AI engines increased, such as ALPS, (EDL?)
			// we should write as many as such converters.
			// How about we unify all featureMetas?
			fms[i] = &featureMeta{
				FeatureName: fr.X[i].FeatureName,
				Dtype:       fr.X[i].Dtype,
				Delimiter:   fr.X[i].Delimiter,
				InputShape:  fr.X[i].InputShape,
				IsSparse:    fr.X[i].IsSparse,
			}
		}
		return fms, fr.Label, nil
	}
	return nil, "", fmt.Errorf("analyzer: model[%s] not supported", pr.estimator)
}

func genAnalyzer(pr *extendedSelect, db *DB, cwd string, modelDir string) (*bytes.Buffer, error) {
	pr, _, err := loadModelMeta(pr, db, cwd, modelDir, pr.trainedModel)
	if err != nil {
		return nil, fmt.Errorf("loadModelMeta %v", err)
	}

	fms, label, err := readFeatureNames(pr, db)
	if err != nil {
		return nil, fmt.Errorf("read feature names err: %v", err)
	}
	fr, err := newAnalyzeFiller(pr, db, fms, label)
	if err != nil {
		return nil, fmt.Errorf("create analyze filler failed: %v", err)
	}

	var program bytes.Buffer
	// FIXME(weiguo): comment the following just for debug. You should not see this.
	// err = analyzeTemplate.Execute(&program, fr)
	err = analyzeTemplate.Execute(os.Stdout, fr)
	return &program, err
}
