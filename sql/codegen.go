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
	"strings"
	"text/template"

	"github.com/go-sql-driver/mysql"
	"sqlflow.org/gohive"
	"sqlflow.org/gomaxcompute"
)

type columnType struct {
	Name string
	Type string
}

type connectionConfig struct {
	Driver   string
	User     string
	Password string
	Host     string
	Port     string
	Database string
	Auth     string // the auth field is only used for hiveserver2
	Session  map[string]string
}

type modelConfig struct {
	EstimatorCode       string
	FeatureColumnParmas string
	AttrParams          string
	BatchSize           int
	Epochs              int
	Save                string
	IsKerasModel        bool
}

type featureMeta struct {
	FeatureName string
	Dtype       string
	Delimiter   string
	InputShape  string
	IsSparse    bool
}

type filler struct {
	IsTrain            bool
	TrainingDataset    string // IsTrain == true
	ValidationDataset  string // IsTrain == true
	PredictionDataset  string // IsTrain != true
	X                  []*featureMeta
	FeatureColumnsCode map[string][]string
	Y                  *featureMeta
	TableName          string
	modelConfig
	connectionConfig
}

// parseModelURI returns isKerasModel, modelClassString
func parseModelURI(modelString string) (bool, string) {
	if strings.HasPrefix(modelString, "sqlflow_models.") {
		return true, modelString
	}
	return false, fmt.Sprintf("tf.estimator.%s", modelString)
}

func trainingAndValidationDataset(pr *extendedSelect, ds *trainAndValDataset) (string, string) {
	if pr.train && ds != nil {
		return fmt.Sprintf("SELECT * FROM %s", ds.training), fmt.Sprintf("SELECT * FROM %s", ds.validation)
	}
	return pr.standardSelect.String(), pr.standardSelect.String()
}

func newFiller(pr *extendedSelect, ds *trainAndValDataset, fts fieldTypes, db *DB) (*filler, error) {
	isKerasModel, modelClassString := parseModelURI(pr.estimator)
	auth := ""
	var sc map[string]string
	if db.driverName == "hive" {
		cfg, err := gohive.ParseDSN(db.dataSourceName)
		if err != nil {
			return nil, err
		}
		auth = cfg.Auth
		sc = cfg.SessionCfg
	}
	training, validation := trainingAndValidationDataset(pr, ds)
	r := &filler{
		IsTrain:           pr.train,
		TrainingDataset:   training,
		ValidationDataset: validation,
		PredictionDataset: pr.standardSelect.String(),
		modelConfig: modelConfig{
			EstimatorCode: modelClassString,
			BatchSize:     1,
			Epochs:        1,
			Save:          pr.save,
			IsKerasModel:  isKerasModel,
		},
		connectionConfig: connectionConfig{
			Auth:    auth,
			Session: sc,
		},
	}

	trainResolved, err := resolveTrainClause(&pr.trainClause)
	if err != nil {
		return nil, err
	}
	r.modelConfig.BatchSize = trainResolved.BatchSize
	r.modelConfig.Epochs = trainResolved.Epoch

	featureColumnsCode := make(map[string][]string)
	for target, columns := range pr.columns {
		feaCols, colSpecs, err := resolveTrainColumns(&columns)
		if err != nil {
			return nil, err
		}
		if len(colSpecs) != 0 {
			return nil, fmt.Errorf("newFiller doesn't support DENSE/SPARSE")
		}
		for _, col := range feaCols {
			feaColCode, e := col.GenerateCode()
			if e != nil {
				return nil, e
			}
			// FIXME(typhoonzero): Use Heuristic rules to determine whether a column should be transformed to a
			// tf.SparseTensor. Currently the rules are:
			// if column have delimiter and it's not a sequence_catigorical_column, we'll treat it as a sparse column
			// else, use dense column.
			isSparse := false
			var isEmb bool
			_, ok := col.(*sequenceCategoryIDColumn)
			if !ok {
				_, isEmb = col.(*embeddingColumn)
				if isEmb {
					_, ok = col.(*embeddingColumn).CategoryColumn.(*sequenceCategoryIDColumn)
				}
			}
			if !ok && col.GetDelimiter() != "" {
				if _, ok := col.(*numericColumn); !ok {
					isSparse = true
				}
			}
			fm := &featureMeta{
				FeatureName: col.GetKey(),
				Dtype:       col.GetDtype(),
				Delimiter:   col.GetDelimiter(),
				InputShape:  col.GetInputShape(),
				IsSparse:    isSparse,
			}
			r.X = append(r.X, fm)
			featureColumnsCode[target] = append(
				featureColumnsCode[target],
				feaColCode)
		}
	}

	// Format estimator creation code
	var attrParams []string
	for _, attrValue := range trainResolved.ModelConstructorParams {
		attrValueStr, err := attrValue.GenerateCode()
		if err != nil {
			return nil, err
		}
		attrParams = append(attrParams, attrValueStr)
	}
	r.AttrParams = strings.Join(attrParams, ",")

	var featureColumnParams []string
	for target, fcCodeList := range featureColumnsCode {
		paramKey := target
		if paramKey == "" {
			paramKey = "feature_columns"
		}
		featureColumnParams = append(
			featureColumnParams,
			fmt.Sprintf("%s=[%s]", paramKey, strings.Join(fcCodeList, ",")),
		)
	}
	r.FeatureColumnParmas = strings.Join(featureColumnParams, ",")

	// Default use int32 label dtype
	labelDtype := "int32"
	if dbDType, ok := fts.get(pr.label); ok {
		v := strings.ToUpper(dbDType)
		if v == "FLOAT" {
			labelDtype = "float32"
		} else if v == "DOUBLE" {
			labelDtype = "float64"
		} else if v == "INT" || v == "INT_TYPE" {
			labelDtype = "int32"
		} else if v == "BIGINT" {
			labelDtype = "int64"
		} else {
			log.Fatalf("Unsupported label data type: %s", v)
		}
	}
	r.Y = &featureMeta{
		FeatureName: pr.label,
		Dtype:       labelDtype,
		Delimiter:   ",",
		InputShape:  "[1]",
		IsSparse:    false,
	}

	var e error
	if !pr.train {
		if r.TableName, _, e = parseTableColumn(pr.into); e != nil {
			return nil, e
		}
	}

	return fillDatabaseInfo(r, db)
}

func fillDatabaseInfo(r *filler, db *DB) (*filler, error) {
	r.Driver = db.driverName
	switch db.driverName {
	case "mysql":
		cfg, err := mysql.ParseDSN(db.dataSourceName)
		if err != nil {
			return nil, err
		}
		sa := strings.Split(cfg.Addr, ":")
		r.Host, r.Port, r.Database = sa[0], sa[1], cfg.DBName
		r.User, r.Password = cfg.User, cfg.Passwd
	case "sqlite3":
		r.Database = db.dataSourceName
	case "hive":
		cfg, err := gohive.ParseDSN(db.dataSourceName)
		if err != nil {
			return nil, err
		}
		sa := strings.Split(cfg.Addr, ":")
		r.Host, r.Port, r.Database = sa[0], sa[1], cfg.DBName
		r.User, r.Password = cfg.User, cfg.Passwd
		// remove the last ';' which leads to a ParseException
		r.TrainingDataset = removeLastSemicolon(r.TrainingDataset)
		r.ValidationDataset = removeLastSemicolon(r.ValidationDataset)
		r.PredictionDataset = removeLastSemicolon(r.PredictionDataset)
	case "maxcompute":
		cfg, err := gomaxcompute.ParseDSN(db.dataSourceName)
		if err != nil {
			return nil, err
		}
		// setting r.Port=0 just makes connect() happy
		r.Host, r.Port, r.Database = cfg.Endpoint, "0", cfg.Project
		r.User, r.Password = cfg.AccessID, cfg.AccessKey
	default:
		return nil, fmt.Errorf("sqlfow currently doesn't support DB %v", db.driverName)
	}
	return r, nil
}

func removeLastSemicolon(s string) string {
	n := len(s)
	if n > 0 && s[n-1] == ';' {
		return s[0 : n-1]
	}
	return s
}

func genTF(w io.Writer, pr *extendedSelect, ds *trainAndValDataset, fts fieldTypes, db *DB) error {
	r, e := newFiller(pr, ds, fts, db)
	if e != nil {
		return e
	}
	if pr.train {
		return tfTrainTemplate.Execute(w, r)
	}
	return tfPredTemplate.Execute(w, r)
}

var tfTrainTemplate = template.Must(template.New("codegenTfTrain").Parse(tfTrainTemplateText))
var tfPredTemplate = template.Must(template.New("codegenTfPred").Parse(tfPredTemplateText))
