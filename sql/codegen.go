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
	"github.com/sql-machine-learning/sqlflow/sql/columns"
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
	Verbose             int
}

// FeatureMeta describes feature column meta data
type FeatureMeta struct {
	FeatureName string
	Dtype       string
	Delimiter   string
	InputShape  string
	IsSparse    bool
}

// Estimator describes estimator meta data
type Estimator struct {
	IsTrain              bool
	TrainingDatasetSQL   string // IsTrain == true
	ValidationDatasetSQL string // IsTrain == true
	PredictionDatasetSQL string // IsTrain != true
	X                    []*FeatureMeta
	Y                    *FeatureMeta
	TableName            string
	*connectionConfig
}

type tfFiller struct {
	Estimator
	modelConfig
	FeatureColumnsCode map[string][]string
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

func newFiller(pr *extendedSelect, ds *trainAndValDataset, fts fieldTypes, db *DB) (*tfFiller, error) {
	isKerasModel, modelClassString := parseModelURI(pr.estimator)
	training, validation := trainingAndValidationDataset(pr, ds)
	r := &tfFiller{
		Estimator: Estimator{
			IsTrain:              pr.train,
			TrainingDatasetSQL:   training,
			ValidationDatasetSQL: validation,
			PredictionDatasetSQL: pr.standardSelect.String(),
		},
		modelConfig: modelConfig{
			EstimatorCode: modelClassString,
			BatchSize:     1,
			Epochs:        1,
			Verbose:       0,
			Save:          pr.save,
			IsKerasModel:  isKerasModel,
		},
	}

	var err error
	r.connectionConfig, err = newConnectionConfig(db)
	if err != nil {
		return nil, err
	}
	if r.Driver == "hive" {
		// remove the last ';' which leads to a (hive)ParseException
		r.TrainingDatasetSQL = strings.TrimSuffix(r.TrainingDatasetSQL, ";")
		r.ValidationDatasetSQL = strings.TrimSuffix(r.ValidationDatasetSQL, ";")
		r.PredictionDatasetSQL = strings.TrimSuffix(r.PredictionDatasetSQL, ";")
	}

	trainResolved, err := resolveTrainClause(&pr.trainClause, &pr.standardSelect, r.connectionConfig)
	if err != nil {
		return nil, err
	}
	r.modelConfig.BatchSize = trainResolved.BatchSize
	r.modelConfig.Epochs = trainResolved.Epoch
	r.modelConfig.Verbose = trainResolved.Verbose

	featureColumnsCode := make(map[string][]string)
	for target, columnsExpr := range pr.columns {
		feaCols, colSpecs, err := resolveTrainColumns(&columnsExpr)
		if err != nil {
			return nil, err
		}
		if len(colSpecs) != 0 {
			return nil, fmt.Errorf("newFiller doesn't support DENSE/SPARSE")
		}
		for _, col := range feaCols {
			// TODO(typhoonzero): pass columnSpecs if needed.
			feaColCode, e := col.GenerateCode(nil)
			if e != nil {
				return nil, e
			}
			if len(feaColCode) > 1 {
				return nil, fmt.Errorf("does not support grouped feature column yet, grouped column: %v", feaColCode)
			}

			// FIXME(typhoonzero): Use Heuristic rules to determine whether a column should be transformed to a
			// tf.SparseTensor. Currently the rules are:
			// if column have delimiter and it's not a sequence_catigorical_column, we'll treat it as a sparse column
			// else, use dense column.
			isSparse := false
			var isEmb bool
			_, ok := col.(*columns.SequenceCategoryIDColumn)
			if !ok {
				_, isEmb = col.(*columns.EmbeddingColumn)
				if isEmb {
					_, ok = col.(*columns.EmbeddingColumn).CategoryColumn.(*columns.SequenceCategoryIDColumn)
				}
			}
			if !ok && col.GetDelimiter() != "" {
				if _, ok := col.(*columns.NumericColumn); !ok {
					isSparse = true
				}
			}
			fm := &FeatureMeta{
				FeatureName: col.GetKey(),
				Dtype:       col.GetDtype(),
				Delimiter:   col.GetDelimiter(),
				InputShape:  col.GetInputShape(),
				IsSparse:    isSparse,
			}
			r.X = append(r.X, fm)
			featureColumnsCode[target] = append(
				featureColumnsCode[target],
				feaColCode[0])
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
	var labelColumnName string
	var e error

	if pr.train {
		labelColumnName = pr.label
	} else {
		r.TableName, labelColumnName, e = parseTableColumn(pr.into)
		if e != nil {
			return nil, fmt.Errorf("invalid predParsed.into, %v", e)
		}
	}

	if dbDType, ok := fts.get(labelColumnName); ok {
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
	r.Y = &FeatureMeta{
		FeatureName: labelColumnName,
		Dtype:       labelDtype,
		Delimiter:   ",",
		InputShape:  "[1]",
		IsSparse:    false,
	}

	return r, e
}

func newConnectionConfig(db *DB) (*connectionConfig, error) {
	cc := &connectionConfig{
		Driver: db.driverName,
	}
	switch db.driverName {
	case "mysql":
		cfg, err := mysql.ParseDSN(db.dataSourceName)
		if err != nil {
			return nil, err
		}
		sa := strings.Split(cfg.Addr, ":")
		cc.Host, cc.Port, cc.Database = sa[0], sa[1], cfg.DBName
		cc.User, cc.Password = cfg.User, cfg.Passwd
	case "sqlite3":
		cc.Database = db.dataSourceName
	case "hive":
		cfg, err := gohive.ParseDSN(db.dataSourceName)
		if err != nil {
			return nil, err
		}
		cc.Auth = cfg.Auth
		cc.Session = cfg.SessionCfg
		sa := strings.Split(cfg.Addr, ":")
		cc.Host, cc.Port, cc.Database = sa[0], sa[1], cfg.DBName
		cc.User, cc.Password = cfg.User, cfg.Passwd
	case "maxcompute":
		cfg, err := gomaxcompute.ParseDSN(db.dataSourceName)
		if err != nil {
			return nil, err
		}
		// setting r.Port=0 just makes connect() happy
		cc.Host, cc.Port, cc.Database = cfg.Endpoint, "0", cfg.Project
		cc.User, cc.Password = cfg.AccessID, cfg.AccessKey
	default:
		return nil, fmt.Errorf("sqlfow currently doesn't support DB %v", db.driverName)
	}
	return cc, nil
}

func genTF(w io.Writer, pr *extendedSelect, ds *trainAndValDataset, fts fieldTypes, db *DB) error {
	r, e := newFiller(pr, ds, fts, db)
	if e != nil {
		return e
	}
	if pr.train {
		return tfTrainTemplate.Execute(w, r)
	}
	if e := createPredictionTable(pr, db); e != nil {
		return fmt.Errorf("failed to create prediction table: %v", e)
	}
	return tfPredTemplate.Execute(w, r)
}

var tfTrainTemplate = template.Must(template.New("codegenTfTrain").Parse(tfTrainTemplateText))
var tfPredTemplate = template.Must(template.New("codegenTfPred").Parse(tfPredTemplateText))
