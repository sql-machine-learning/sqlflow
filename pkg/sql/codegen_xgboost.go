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
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/template"

	"github.com/asaskevich/govalidator"
	pb "sqlflow.org/sqlflow/pkg/server/proto"
)

type xgbTrainConfig struct {
	NumBoostRound       int
	Maximize            bool
	EarlyStoppingRounds int
}

type xgbFiller struct {
	Estimator
	xgbTrainConfig
	Save             string
	ParamsCfgJSON    string
	HDFSNameNodeAddr string
	HiveLocation     string
}

func resolveTrainCfg(attrs map[string]*attribute) *xgbTrainConfig {
	return &xgbTrainConfig{
		NumBoostRound:       getIntAttr(attrs, "train.num_boost_round", 10),
		Maximize:            getBoolAttr(attrs, "train.maximize", false, false),
		EarlyStoppingRounds: getIntAttr(attrs, "train.early_stopping_rounds", -1),
	}
}

func resolveParamsCfg(attrs map[string]*attribute) (map[string]interface{}, error) {
	// extract the attributes without any prefix as the XGBoost Parmaeters
	params := make(map[string]interface{})
	var err error
	for k, v := range attrs {
		if !strings.Contains(k, ".") {
			var vStr string
			var ok bool
			if vStr, ok = v.Value.(string); !ok {
				return nil, fmt.Errorf("convert params %s to string failed, %v", vStr, err)
			}
			if govalidator.IsFloat(vStr) {
				floatVal, err := strconv.ParseFloat(vStr, 16)
				if err != nil {
					return nil, fmt.Errorf("convert params %s to float32 failed, %v", vStr, err)
				}
				params[k] = floatVal
			} else if govalidator.IsInt(vStr) {
				if params[k], err = strconv.ParseInt(vStr, 0, 32); err != nil {
					return nil, fmt.Errorf("convert params %s to int32 failed, %v", vStr, err)
				}
			} else if govalidator.IsASCII(vStr) {
				params[k] = vStr
			} else {
				return nil, fmt.Errorf("unsupported params type: %s", vStr)
			}
		}
	}
	return params, nil
}

func resolveModelName(pr *extendedSelect) (string, error) {
	estimatorParts := strings.Split(pr.estimator, ".")
	if len(estimatorParts) != 2 {
		return "", fmt.Errorf("XGBoost Estimator should be xgboost.modelname, current: %s", pr.estimator)
	}
	if strings.ToUpper(estimatorParts[1]) != "GBTREE" {
		return "", fmt.Errorf("model name %s is not supported yet", estimatorParts[1])
	}
	return estimatorParts[1], nil
}

func newXGBFiller(pr *extendedSelect, ds *trainAndValDataset, db *DB, session *pb.Session) (*xgbFiller, error) {
	attrs, err := resolveAttribute(&pr.trainAttrs)
	if err != nil {
		return nil, err
	}
	training, validation := trainingAndValidationDataset(pr, ds)
	isTrain := pr.train
	r := &xgbFiller{
		Estimator: Estimator{
			IsTrain:              pr.train,
			TrainingDatasetSQL:   training,
			ValidationDatasetSQL: validation,
		},
		xgbTrainConfig:   *resolveTrainCfg(attrs),
		Save:             pr.save,
		HDFSNameNodeAddr: session.GetHdfsNamenodeAddr(),
		HiveLocation:     session.GetHiveLocation(),
	}
	if !isTrain && !pr.analyze {
		r.PredictionDatasetSQL = pr.standardSelect.String()
		if r.TableName, _, err = parseTableColumn(pr.into); err != nil {
			return nil, err
		}
		r.Save = pr.model
	}

	if isTrain {
		objective := getStringAttr(attrs, "objective", "gbtree")
		// resolve the attribute keys without any prefix as the XGBoost Paremeters
		params, err := resolveParamsCfg(attrs)
		if err != nil {
			return nil, err
		}
		params["objective"] = objective

		// get model name, could be gbtree, gblinear or dart.
		// TODO(typhoonzero): only gbtree is supported here, use model name to generate
		// differnet training code.
		_, err = resolveModelName(pr)
		if err != nil {
			return nil, err
		}

		paramsJSON, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		r.ParamsCfgJSON = string(paramsJSON)
	}

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
			fm := &FeatureMeta{
				FeatureName: col.GetKey(),
				Dtype:       col.GetDtype(),
				Delimiter:   col.GetDelimiter(),
				InputShape:  col.GetInputShape(),
				IsSparse:    false,
			}
			r.X = append(r.X, fm)
		}
	}
	r.Y = &FeatureMeta{
		FeatureName: pr.label,
		Dtype:       "int32",
		Delimiter:   ",",
		InputShape:  "[1]",
		IsSparse:    false,
	}

	return r, nil
}

func genXGBoost(w io.Writer, pr *extendedSelect, ds *trainAndValDataset, fts fieldTypes, db *DB, session *pb.Session) error {
	r, e := newXGBFiller(pr, ds, db, session)
	if e != nil {
		return e
	}
	if pr.train {
		return xgbTrainTemplate.Execute(w, r)
	}
	if e := createPredictionTable(pr, db, session); e != nil {
		return fmt.Errorf("failed to create prediction table: %v", e)
	}
	return xgbPredictTemplate.Execute(w, r)
}

var xgbTrainTemplate = template.Must(template.New("codegenXGBTrain").Parse(xgbTrainTemplateText))
var xgbPredictTemplate = template.Must(template.New("codegenXGBPredict").Parse(xgbPredictTemplateText))
