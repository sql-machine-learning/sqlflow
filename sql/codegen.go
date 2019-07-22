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
	"github.com/go-sql-driver/mysql"
	"io"
	"sqlflow.org/gohive"
	"sqlflow.org/gomaxcompute"
	"strings"
)

type columnType struct {
	Name string
	Type string
}

type connectionConfig struct {
	User     string
	Password string
	Host     string
	Port     string
	Database string
}

type modelConfig struct {
	Estimator    string
	Attrs        map[string]string
	Save         string
	IsKerasModel bool
}

type featureMeta struct {
	FeatureName string
	Dtype       string
	Delimiter   string
	InputShape  string
	IsSparse    bool
}

type filler struct {
	IsTrain        bool
	Driver         string
	StandardSelect string
	X              []*featureMeta
	// key: for target (e.g. deep-wide model), value: list of generated code for current target
	FeatureColumnsCode map[string][]string
	Y                  *featureMeta
	TableName          string
	modelConfig
	connectionConfig
}

type estimatorTemplate interface {
	execute(w io.Writer, r *filler) error
}

func translateColumnToFeature(fts *fieldTypes, driverName, ident string) (*columnType, error) {
	ct, ok := fts.get(ident)
	if !ok {
		return nil, fmt.Errorf("codegen: Cannot find type of field %s", ident)
	}
	ctype, e := universalizeColumnType(driverName, ct)
	if e != nil {
		return nil, e
	}
	ctype = strings.ToUpper(ctype)

	if ctype == "FLOAT" || ctype == "INT" || ctype == "DOUBLE" || ctype == "BIGINT" {
		return &columnType{ident, "numeric_column"}, nil
	} else if ctype == "TEXT" || ctype == "VARCHAR" {
		// FIXME(typhoonzero): only support preprocessed string of int vector
		// like: "231,291,0,0,9", to support arbitrary string, we need to provide
		// additional information like how to parse.
		// TODO(typhoonzero): need to support categorical_column_with_vocabulary_list
		// which read vocabulary from DB.
		// return &columnType{ident, "categorical_column_with_identity"}, nil
		return &columnType{ident, "categorical_column_with_identity"}, nil
	}
	return nil, fmt.Errorf("unsupported type %s of field %s", ctype, ident)
}

// parseModelURI returns isKerasModel, modelClassString
func parseModelURI(modelString string) (bool, string) {
	if strings.HasPrefix(modelString, "sqlflow_models.") {
		return true, modelString
	}
	return false, fmt.Sprintf("tf.estimator.%s", modelString)
}

// TODO(weiguo): fts -> pointer
func newFiller(pr *extendedSelect, fts fieldTypes, db *DB) (*filler, error) {
	isKerasModel, modelClassString := parseModelURI(pr.estimator)
	r := &filler{
		IsTrain:        pr.train,
		StandardSelect: pr.standardSelect.String(),
		modelConfig: modelConfig{
			Estimator:    modelClassString,
			Attrs:        make(map[string]string),
			Save:         pr.save,
			IsKerasModel: isKerasModel,
		},
	}
	for k, v := range pr.attrs {
		r.Attrs[k] = v.String()
	}

	for target, columns := range pr.columns {
		feaCols, colSpecs, err := resolveTrainColumns(&columns)
		if err != nil {
			return nil, err
		}
		if len(colSpecs) != 0 {
			return nil, fmt.Errorf("newFiller doesn't support DENSE/SPARSE")
		}
		r.FeatureColumnsCode = make(map[string][]string)
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
			r.FeatureColumnsCode[target] = append(
				r.FeatureColumnsCode[target],
				feaColCode)
		}
	}

	// FIXME(typhoonzero): support soft label in addition to int types
	r.Y = &featureMeta{
		FeatureName: pr.label,
		Dtype:       "int",
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
		r.StandardSelect = removeLastSemicolon(r.StandardSelect)
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

func codegen(w io.Writer, pr *extendedSelect, fts fieldTypes, db *DB) error {
	r, e := newFiller(pr, fts, db)
	if e != nil {
		return e
	}

	estimatorKey := strings.ToUpper(r.Estimator)
	// select estimatorTemplate
	var temp estimatorTemplate
	switch {
	case strings.HasPrefix(estimatorKey, "TF.ESTIMATOR") || r.IsKerasModel:
		temp = &tfTemplate{}
	case strings.HasPrefix(estimatorKey, "XGBOOST"):
		temp = &xgboostTemplate{}
	default:
		return fmt.Errorf("unknown estimator name: %s", estimatorKey)
	}

	if e = temp.execute(w, r); e != nil {
		return fmt.Errorf("codegen: failed executing template: %v", e)
	}
	return nil
}
