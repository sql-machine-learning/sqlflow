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
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
	"sqlflow.org/gohive"
	"sqlflow.org/gomaxcompute"
)

type xgboostFiller struct {
	modelPath string
	xgboostFields
	xgColumnFields
	xgDataSourceFields
	xgboostJSON      string
	xgDataSourceJSON string
	xgColumnJSON     string
}

type xgboostFields struct {
	NumRound        uint `json:"num_boost_round,omitempty"`
	AutoTrain       bool `json:"auto_train,omitempty"`
	xgBoosterFields `json:"params,omitempty"`
}

type xgBoosterFields struct {
	Objective           string  `json:"objective,omitempty"`
	Booster             string  `json:"booster,omitempty"`
	NumClass            uint    `json:"num_class,omitempty"`
	MaxDepth            uint    `json:"max_depth,omitempty"`
	Eta                 float32 `json:"eta,omitempty"`
	TreeMethod          string  `json:"tree_method,omitempty"`
	EvalMetric          string  `json:"eval_metric,omitempty"`
	Subsample           float32 `json:"subsample,omitempty"`
	ColSampleByTree     float32 `json:"colsample_bytree,omitempty"`
	ColSampleByLevel    float32 `json:"colsample_bylevel,omitempty"`
	MaxBin              uint    `json:"max_bin,omitempty"`
	ConvergenceCriteria string  `json:"convergence_criteria,omitempty"`
	Verbosity           uint    `json:"verbosity,omitempty"`
}

type xgColumnFields struct {
	Label                string   `json:"label,omitempty"`
	Group                string   `json:"group,omitempty"`
	Weight               string   `json:"weight,omitempty"`
	AppendColumns        []string `json:"append_columns,omitempty"`
	xgFeatureFields      `json:"features,omitempty"`
	xgResultColumnFields `json:"result_columns,omitempty"`
}

type xgResultColumnFields struct {
	ResultColumn   string `json:"result_column,omitempty"`
	ProbColumn     string `json:"probability_column,omitempty"`
	DetailColumn   string `json:"detail_column,omitempty"`
	EncodingColumn string `json:"leaf_column,omitempty"`
}

type xgFeatureFields struct {
	FeatureColumns []string `json:"columns,omitempty"`
	IsSparse       bool     `json:"is_sparse,omitempty"`
	Delimiter      string   `json:"item_delimiter,omitempty"`
	FeatureSize    uint     `json:"feature_num,omitempty"`
}

type xgDataSourceFields struct {
	IsTrain                bool             `json:"is_train,omitempty"`
	StandardSelect         string           `json:"standard_select,omitempty"`
	IsTensorFlowIntegrated bool             `json:"is_tf_integrated,omitempty"`
	X                      []*xgFeatureMeta `json:"x,omitempty"`
	LabelField             *xgFeatureMeta   `json:"label,omitempty"`
	WeightField            *xgFeatureMeta   `json:"weight,omitempty"`
	GroupField             *xgFeatureMeta   `json:"group,omitempty"`
	xgDataBaseField        `json:"db_config,omitempty"`
	WriteBatchSize         int `json:"write_batch_size,omitempty"`
}

type xgDataBaseField struct {
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
	Host     string `json:"host,omitempty"`
	Port     string `json:"port,omitempty"`
	Database string `json:"database,omitempty"`
	Driver   string `json:"driver,omitempty"`
}

type xgFeatureMeta struct {
	FeatureName       string `json:"feature_name,omitempty"`
	Dtype             string `json:"dtype,omitempty"`
	Delimiter         string `json:"delimiter,omitempty"`
	InputShape        string `json:"shape,omitempty"`
	IsSparse          bool   `json:"is_sparse,omitempty"`
	FeatureColumnCode string `json:"fc_code,omitempty"`
}

func xgParseAttrError(err error) error {
	return fmt.Errorf("xgParseAttrError: %v", err)
}

func xgParseColumnError(tpe string, err error) error {
	return fmt.Errorf("xgParseColumnError: column type(%s), error: %v", tpe, err)
}

func xgParseEstimatorError(tpe string, err error) error {
	return fmt.Errorf("xgParseEstimatorError: Estimator keyword(%s), error: %v", tpe, err)
}

func xgInvalidAttrValError(key string, val []string) error {
	return fmt.Errorf("xgInvalidAttrValError: invalid attr value(%v) for key(%s)", val, key)
}

func xgDupAttrSettingError(key string) error {
	return fmt.Errorf("xgDupAttrSettingError: duplicate attr setting, key is %s", key)
}

func xgMixSchemaError() error {
	return fmt.Errorf("xgMixSchemaError: SPARSE column can't work with other columns")
}

func xgMultiSparseError(colNames []string) error {
	return fmt.Errorf("xgMultiSparseError: SPARSE column should be unique, but found more than one: %v", colNames)
}

func xgUnknownFCError(kw string) error {
	return fmt.Errorf("xgUnknownFCError: feature column keyword(`%s`) is not supported by xgboost engine", kw)
}

func xgUnsupportedColTagError() error {
	return fmt.Errorf("xgUnsupportedColTagError: valid column tags of xgboost engine([feature_columns, group, weight])")
}

func uIntPartial(key string, ptrFn func(*xgboostFiller) *uint) func(*map[string][]string, *xgboostFiller) error {
	return func(a *map[string][]string, r *xgboostFiller) error {
		// xgParseAttr will ensure the key is existing in map
		val, _ := (*a)[key]
		if len(val) != 1 {
			return xgInvalidAttrValError(key, val)
		}
		if intVal, err := strconv.ParseUint(val[0], 10, 32); err != nil {
			return xgInvalidAttrValError(key, val)
		} else if intPtr := ptrFn(r); *intPtr != 0 {
			return xgDupAttrSettingError(key)
		} else {
			*intPtr = uint(intVal)
			delete(*a, key)
		}
		return nil
	}
}

func fp32Partial(key string, ptrFn func(*xgboostFiller) *float32) func(*map[string][]string, *xgboostFiller) error {
	return func(a *map[string][]string, r *xgboostFiller) error {
		// xgParseAttr will ensure the key is existing in map
		val, _ := (*a)[key]
		if len(val) != 1 {
			return xgInvalidAttrValError(key, val)
		}
		if fpVal, err := strconv.ParseFloat(val[0], 32); err != nil {
			return xgInvalidAttrValError(key, val)
		} else if fpPtr := ptrFn(r); *fpPtr != 0 {
			return xgDupAttrSettingError(key)
		} else {
			*fpPtr = float32(fpVal)
			delete(*a, key)
		}
		return nil
	}
}

func boolPartial(key string, ptrFn func(*xgboostFiller) *bool) func(*map[string][]string, *xgboostFiller) error {
	return func(a *map[string][]string, r *xgboostFiller) error {
		// xgParseAttr will ensure the key is existing in map
		val, _ := (*a)[key]
		if len(val) != 1 {
			return xgInvalidAttrValError(key, val)
		}
		bVal, err := strconv.ParseBool(val[0])
		if err != nil {
			return xgInvalidAttrValError(key, val)
		}
		bPtr := ptrFn(r)
		*bPtr = bVal
		delete(*a, key)
		return nil
	}
}

func strPartial(key string, ptrFn func(*xgboostFiller) *string) func(*map[string][]string, *xgboostFiller) error {
	return func(a *map[string][]string, r *xgboostFiller) error {
		// xgParseAttr will ensure the key is existing in map
		val, _ := (*a)[key]
		if len(val) != 1 {
			return xgInvalidAttrValError(key, val)
		}
		stringPtr := ptrFn(r)
		if len(*stringPtr) != 0 {
			return xgDupAttrSettingError(key)
		}
		*stringPtr = val[0]
		delete(*a, key)
		return nil
	}
}

func sListPartial(key string, ptrFn func(*xgboostFiller) *[]string) func(*map[string][]string, *xgboostFiller) error {
	return func(a *map[string][]string, r *xgboostFiller) error {
		// xgParseAttr will ensure the key is existing in map
		val, _ := (*a)[key]
		strListPtr := ptrFn(r)
		if len(*strListPtr) != 0 {
			return xgDupAttrSettingError(key)
		}
		*strListPtr = val
		delete(*a, key)
		return nil
	}
}

var xgbTrainAttrSetterMap = map[string]func(*map[string][]string, *xgboostFiller) error{
	// booster params
	"objective":            strPartial("objective", func(r *xgboostFiller) *string { return &(r.Objective) }),
	"booster":              strPartial("booster", func(r *xgboostFiller) *string { return &(r.Booster) }),
	"max_depth":            uIntPartial("max_depth", func(r *xgboostFiller) *uint { return &(r.MaxDepth) }),
	"num_class":            uIntPartial("num_class", func(r *xgboostFiller) *uint { return &(r.NumClass) }),
	"eta":                  fp32Partial("eta", func(r *xgboostFiller) *float32 { return &(r.Eta) }),
	"tree_method":          strPartial("tree_method", func(r *xgboostFiller) *string { return &(r.TreeMethod) }),
	"eval_metric":          strPartial("eval_metric", func(r *xgboostFiller) *string { return &(r.EvalMetric) }),
	"subsample":            fp32Partial("subsample", func(r *xgboostFiller) *float32 { return &(r.Subsample) }),
	"colsample_bytree":     fp32Partial("colsample_bytree", func(r *xgboostFiller) *float32 { return &(r.ColSampleByTree) }),
	"colsample_bylevel":    fp32Partial("colsample_bylevel", func(r *xgboostFiller) *float32 { return &(r.ColSampleByLevel) }),
	"max_bin":              uIntPartial("max_bin", func(r *xgboostFiller) *uint { return &(r.MaxBin) }),
	"convergence_criteria": strPartial("convergence_criteria", func(r *xgboostFiller) *string { return &(r.ConvergenceCriteria) }),
	"verbosity":            uIntPartial("verbosity", func(r *xgboostFiller) *uint { return &(r.Verbosity) }),
	// xgboost train controllers
	"num_round":  uIntPartial("num_round", func(r *xgboostFiller) *uint { return &(r.NumRound) }),
	"auto_train": boolPartial("auto_train", func(r *xgboostFiller) *bool { return &(r.AutoTrain) }),
	// Label, Group, Weight and xgFeatureFields are parsed from columnClause
}

var xgbPredAttrSetterMap = map[string]func(*map[string][]string, *xgboostFiller) error{
	// xgboost output columns (for prediction)
	"append_columns":  sListPartial("append_columns", func(r *xgboostFiller) *[]string { return &(r.AppendColumns) }),
	"result_column":   strPartial("result_column", func(r *xgboostFiller) *string { return &(r.ResultColumn) }),
	"prob_column":     strPartial("prob_column", func(r *xgboostFiller) *string { return &(r.ProbColumn) }),
	"detail_column":   strPartial("detail_column", func(r *xgboostFiller) *string { return &(r.DetailColumn) }),
	"encoding_column": strPartial("encoding_column", func(r *xgboostFiller) *string { return &(r.EncodingColumn) }),
	// Label, Group, Weight and xgFeatureFields are parsed from columnClause
}

func xgParseAttr(pr *extendedSelect, r *xgboostFiller) error {
	var rawAttrs map[string]*expr
	if pr.train {
		rawAttrs = pr.trainAttrs
	} else {
		rawAttrs = pr.predAttrs
	}

	// parse pr.attrs to map[string][]string
	attrs := make(map[string][]string)
	for k, exp := range rawAttrs {
		strExp := exp.String()
		if strings.HasPrefix(strExp, "[") && strings.HasSuffix(strExp, "]") {
			attrs[k] = exp.cdr()
		} else {
			attrs[k] = []string{strExp}
		}
		for i, s := range attrs[k] {
			if s[0] == 34 && s[len(s)-1] == 34 {
				s = s[1 : len(s)-1]
				attrs[k][i] = s
			}
		}
	}

	// fill xgboostFiller with attrs
	var setterMap map[string]func(*map[string][]string, *xgboostFiller) error
	if pr.train {
		setterMap = xgbTrainAttrSetterMap
	} else {
		setterMap = xgbPredAttrSetterMap
	}
	for k := range attrs {
		if setter, ok := setterMap[k]; ok {
			if e := setter(&attrs, r); e != nil {
				return xgParseAttrError(e)
			}
		}
	}

	// remaining elements in xgbAttrs are unsolved ones, so we throw exception if any elements remaining.
	if len(attrs) > 0 {
		for k, v := range attrs {
			log.Errorf("unsolved xgboost attr: %s = %s", k, v)
		}
		return fmt.Errorf("found unsolved xgboost attributes")
	}

	return nil
}

//  parseFeatureColumns, parse feature columns from AST(pr.columns).
//  Features columns are columns owned by default column target whose key is "feature_columns".
//  For now, two schemas are supported:
//	1. sparse-kv
//		schema: COLUMN SPARSE([feature_column], [1-dim shape], [single char delimiter])
//		data example: COLUMN SPARSE("0:1.5 1:100.1f 11:-1.2", [20], " ")
//	2. tf feature columns
//		Roughly same as TFEstimator, except output shape of feaColumns are required to be 1-dim.
func parseFeatureColumns(columns *exprlist, r *xgboostFiller) error {
	feaCols, colSpecs, err := resolveTrainColumns(columns)
	if err != nil {
		return err
	}
	r.IsTensorFlowIntegrated = true
	if len(colSpecs) != 0 {
		if len(feaCols) != 0 {
			return xgMixSchemaError()
		}
		return parseSparseKeyValueFeatures(colSpecs, r)
	}
	if e := parseDenseFeatures(feaCols, r); e != nil {
		return e
	}

	return nil
}

// parseSparseKeyValueFeatures, parse features which is identified by `SPARSE`.
// ex: SPARSE(col1, [100], comma)
func parseSparseKeyValueFeatures(colSpecs []*columnSpec, r *xgboostFiller) error {
	var colNames []string
	for _, spec := range colSpecs {
		colNames = append(colNames, spec.ColumnName)
	}
	if len(colSpecs) > 1 {
		return xgMultiSparseError(colNames)
	}
	spec := colSpecs[0]
	if !spec.IsSparse {
		return xgUnknownFCError("DENSE")
	}
	if len(spec.Shape) != 1 || spec.Shape[0] <= 0 {
		return fmt.Errorf("dim of SPARSE (key-value) column should be one")
	}
	if len(spec.Delimiter) != 1 {
		return fmt.Errorf("invalid demiliter: %s, it should be single char", spec.Delimiter)
	}
	r.IsSparse = true
	r.Delimiter = spec.Delimiter
	r.FeatureSize = uint(spec.Shape[0])
	r.FeatureColumns = []string{spec.ColumnName}
	// We set FeatureMeta like below, to make sure `db_generator` return the feature string directly.
	r.X = []*xgFeatureMeta{
		{
			FeatureName:       spec.ColumnName,
			Dtype:             "string",
			Delimiter:         "",
			InputShape:        "",
			IsSparse:          false,
			FeatureColumnCode: "",
		},
	}
	r.IsTensorFlowIntegrated = false

	return nil
}

// check whether column is raw column (no tf transformation need)
func isSimpleColumn(col featureColumn) bool {
	if _, ok := col.(*numericColumn); ok {
		return col.GetDelimiter() == "" && col.GetInputShape() == "[1]" && col.GetDtype() == "float32"
	}
	return false
}

func parseDenseFeatures(feaCols []featureColumn, r *xgboostFiller) error {
	allSimpleCol := true
	for _, col := range feaCols {
		if allSimpleCol && !isSimpleColumn(col) {
			allSimpleCol = false
		}

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

		feaColCode, e := col.GenerateCode()
		if e != nil {
			return e
		}

		fm := &xgFeatureMeta{
			FeatureName:       col.GetKey(),
			Dtype:             col.GetDtype(),
			Delimiter:         col.GetDelimiter(),
			InputShape:        col.GetInputShape(),
			FeatureColumnCode: feaColCode,
			IsSparse:          isSparse,
		}
		r.X = append(r.X, fm)
	}

	r.Delimiter = ""
	r.IsSparse = false
	r.FeatureSize = uint(0)
	r.FeatureColumns = []string{}
	r.IsTensorFlowIntegrated = true
	if allSimpleCol {
		for _, fm := range r.X {
			fm.FeatureColumnCode = ""
			r.FeatureColumns = append(r.FeatureColumns, fm.FeatureName)
		}
		r.FeatureSize = uint(len(r.X))
		r.IsTensorFlowIntegrated = false
	}

	return nil
}

// parse single simple column
// 		valid schema: COLUMN label_data FOR label
// 		invalid schema: COLUMN DENSE(label_data)
// 		invalid schema: COLUMN label1, label2, label3
// 		invalid schema: NUMERIC(label, 1)
func parseSimpleColumn(field string, columns *exprlist) (*xgFeatureMeta, error) {
	errFn := func(errString string) error {
		return fmt.Errorf("field %s required a simple column, %s", field, errString)
	}
	fcs, css, err := resolveTrainColumns(columns)
	if err != nil {
		return nil, err
	}
	if len(css) > 0 {
		return nil, errFn("but found (DENSE/SPARSE) columns")
	}
	if len(fcs) > 1 {
		return nil, errFn("but found more than one columns")
	}
	if len(fcs) == 0 {
		return nil, errFn("but found no column")
	}
	if !isSimpleColumn(fcs[0]) {
		return nil, errFn("but found TensorFlow feature columns")
	}
	fm := &xgFeatureMeta{
		FeatureName: fcs[0].GetKey(),
	}
	return fm, nil
}

func xgParseColumns(pr *extendedSelect, filler *xgboostFiller) error {
	for target, columns := range pr.columns {
		switch target {
		case "feature_columns":
			if e := parseFeatureColumns(&columns, filler); e != nil {
				return xgParseColumnError(target, e)
			}
		case "group":
			if !pr.train {
				continue
			}
			colMeta, e := parseSimpleColumn("group", &columns)
			if e != nil {
				return xgParseColumnError(target, e)
			}
			filler.GroupField = colMeta
			filler.Group = colMeta.FeatureName
		case "weight":
			if !pr.train {
				continue
			}
			colMeta, e := parseSimpleColumn("weight", &columns)
			if e != nil {
				return xgParseColumnError(target, e)
			}
			filler.WeightField = colMeta
			filler.Weight = colMeta.FeatureName
		default:
			return xgParseColumnError(target, xgUnsupportedColTagError())
		}
	}
	// in predict mode, ignore label info
	if pr.train {
		filler.LabelField = &xgFeatureMeta{
			FeatureName: pr.label,
		}
		filler.Label = pr.label
	}

	return nil
}

func xgParseEstimator(pr *extendedSelect, filler *xgboostFiller) error {
	switch strings.ToUpper(pr.estimator) {
	case "XGBOOSTESTIMATOR":
		if len(filler.Objective) == 0 {
			return xgParseEstimatorError(pr.estimator, fmt.Errorf("objective must be defined"))
		}
	case "XGBOOSTCLASSIFIER":
		if obj := filler.Objective; len(obj) == 0 {
			filler.Objective = "binary:logistic"
		} else if !strings.HasPrefix(obj, "binary") && !strings.HasPrefix(obj, "multi") {
			return xgParseEstimatorError(pr.estimator, fmt.Errorf("found non classification objective(%s)", obj))
		}
	case "XGBOOSTBINARYCLASSIFIER":
		if obj := filler.Objective; len(obj) == 0 {
			filler.Objective = "binary:logistic"
		} else if !strings.HasPrefix(obj, "binary") {
			return xgParseEstimatorError(pr.estimator, fmt.Errorf("found non binary objective(%s)", obj))
		}
	case "XGBOOSTMULTICLASSIFIER":
		if obj := filler.Objective; len(obj) == 0 {
			filler.Objective = "multi:softmax"
		} else if !strings.HasPrefix(obj, "multi") {
			return xgParseEstimatorError(pr.estimator, fmt.Errorf("found non multi-class objective(%s)", obj))
		}
	case "XGBOOSTREGRESSOR":
		if obj := filler.Objective; len(obj) == 0 {
			filler.Objective = "reg:squarederror"
		} else if !strings.HasPrefix(obj, "reg") && !strings.HasPrefix(obj, "rank") {
			return xgParseEstimatorError(pr.estimator, fmt.Errorf("found non reg objective(%s)", obj))
		}
	default:
		return xgParseEstimatorError(pr.estimator, fmt.Errorf("unknown xgboost estimator"))
	}

	return nil
}

func newXGBoostFiller(pr *extendedSelect, fts fieldTypes, db *DB) (*xgboostFiller, error) {
	filler := &xgboostFiller{
		modelPath: pr.save,
	}
	filler.IsTrain = pr.train
	filler.StandardSelect = pr.standardSelect.String()

	// solve keyword: WITH (attributes)
	if e := xgParseAttr(pr, filler); e != nil {
		return nil, fmt.Errorf("failed to set xgboost attributes: %v", e)
	}

	// solve keyword: TRAIN (estimator)
	if e := xgParseEstimator(pr, filler); e != nil {
		return nil, e
	}

	// solve keyword: COLUMN (column clauses)
	if e := xgParseColumns(pr, filler); e != nil {
		return nil, e
	}

	// fill data base info
	if _, e := xgFillDatabaseInfo(filler, db); e != nil {
		return nil, e
	}

	// serialize fields
	jsonBuffer, e := json.Marshal(filler.xgboostFields)
	if e != nil {
		return nil, e
	}
	filler.xgboostJSON = string(jsonBuffer)

	jsonBuffer, e = json.Marshal(filler.xgDataSourceFields)
	if e != nil {
		return nil, e
	}
	filler.xgDataSourceJSON = string(jsonBuffer)

	jsonBuffer, e = json.Marshal(filler.xgColumnFields)
	if e != nil {
		return nil, e
	}
	filler.xgColumnJSON = string(jsonBuffer)

	return filler, nil
}

func xgFillDatabaseInfo(r *xgboostFiller, db *DB) (*xgboostFiller, error) {
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
