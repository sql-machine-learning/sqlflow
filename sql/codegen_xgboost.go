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
	"text/template"
)

type xgbTrainConfig struct {
	NumBoostRound int  `json:"num_boost_round,omitempty"`
	Maximize      bool `json:"maximize,omitempty"`
}

type xgbFiller struct {
	IsTrain              bool
	TrainingDatasetSQL   string
	ValidationDatasetSQL string
	TrainCfg             *xgbTrainConfig
	Features             []*featureMeta
	Label                *featureMeta
	Save                 string
	ParamsCfgJSON        string
	TrainCfgJSON         string
	*connectionConfig
}

func fillXGBTrainCfg(rt *resolvedXGBTrainClause) (*xgbTrainConfig, error) {
	// TODO(Yancey1989): fill all the training control parameters
	c := &xgbTrainConfig{
		NumBoostRound: rt.NumBoostRound,
		Maximize:      rt.Maximize,
	}
	return c, nil
}

func newXGBFiller(pr *extendedSelect, ds *trainAndValDataset, fts fieldTypes, db *DB) (*xgbFiller, error) {
	rt, err := resolveXGBTrainClause(&pr.trainClause)
	training, validation := trainingAndValidationDataset(pr, ds)
	if err != nil {
		return nil, err
	}

	trainCfg, err := fillXGBTrainCfg(rt)
	if err != nil {
		return nil, err
	}

	r := &xgbFiller{
		IsTrain:              pr.train,
		TrainCfg:             trainCfg,
		TrainingDatasetSQL:   training,
		ValidationDatasetSQL: validation,
		Save:                 pr.save,
	}
	// TODO(Yancey1989): fill the train_args and parameters by WITH statment
	r.TrainCfgJSON = ""
	r.ParamsCfgJSON = ""

	if r.connectionConfig, err = newConnectionConfig(db); err != nil {
		return nil, err
	}

	for _, columns := range pr.columns {
		feaCols, colSpecs, err := resolveTrainColumns(&columns)
		if err != nil {
			return nil, err
		}
		if len(colSpecs) != 0 {
			return nil, fmt.Errorf("newXGBoostFiller doesn't support DENSE/SPARSE")
		}
		for _, col := range feaCols {
			fm := &featureMeta{
				FeatureName: col.GetKey(),
				Dtype:       col.GetDtype(),
				Delimiter:   col.GetDelimiter(),
				InputShape:  col.GetInputShape(),
				IsSparse:    false,
			}
			r.Features = append(r.Features, fm)
		}
	}
	r.Label = &featureMeta{
		FeatureName: pr.label,
		Dtype:       "int32",
		Delimiter:   ",",
		InputShape:  "[1]",
		IsSparse:    false,
	}

	return r, nil
}

func genXGBoost(w io.Writer, pr *extendedSelect, ds *trainAndValDataset, fts fieldTypes, db *DB) error {
	r, e := newXGBFiller(pr, ds, fts, db)
	if e != nil {
		return e
	}
	if pr.train {
		return xgbTrainTemplate.Execute(w, r)
	}
	return fmt.Errorf("xgboost prediction codegen has not been implemented")
}

var xgbTrainTemplate = template.Must(template.New("codegenXGBTrain").Parse(xgbTrainTemplateText))
