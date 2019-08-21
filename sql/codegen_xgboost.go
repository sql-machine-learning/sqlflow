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
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-sql-driver/mysql"
	"sqlflow.org/gohive"
	"sqlflow.org/gomaxcompute"
)

type xgboostFiller struct {
	ModelPath string
	xgLearningFields
	xgColumnFields
	xgDataSourceFields
	LearningJSON   string
	DataSourceJSON string
	ColumnJSON     string
}

type xgLearningFields struct {
	NumRound        uint `json:"num_boost_round,omitempty"`
	AutoTrain       bool `json:"auto_train"`
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
	IsSparse       bool     `json:"is_sparse"`
	Delimiter      string   `json:"item_delimiter,omitempty"`
	FeatureSize    uint     `json:"feature_num,omitempty"`
}

type xgDataSourceFields struct {
	IsTrain                bool             `json:"is_train"`
	StandardSelect         string           `json:"standard_select,omitempty"`
	IsTensorFlowIntegrated bool             `json:"is_tf_integrated"`
	X                      []*xgFeatureMeta `json:"x,omitempty"`
	LabelField             *xgFeatureMeta   `json:"label,omitempty"`
	WeightField            *xgFeatureMeta   `json:"weight,omitempty"`
	GroupField             *xgFeatureMeta   `json:"group,omitempty"`
	xgDataBaseField        `json:"db_config,omitempty"`
	OutputTable            string `json:"output_table,omitempty"`
	WriteBatchSize         int    `json:"write_batch_size,omitempty"`
}

type xgDataBaseField struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Database string `json:"database"`
	Driver   string `json:"driver"`
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
	"train.objective":            strPartial("train.objective", func(r *xgboostFiller) *string { return &(r.Objective) }),
	"train.booster":              strPartial("train.booster", func(r *xgboostFiller) *string { return &(r.Booster) }),
	"train.max_depth":            uIntPartial("train.max_depth", func(r *xgboostFiller) *uint { return &(r.MaxDepth) }),
	"train.num_class":            uIntPartial("train.num_class", func(r *xgboostFiller) *uint { return &(r.NumClass) }),
	"train.eta":                  fp32Partial("train.eta", func(r *xgboostFiller) *float32 { return &(r.Eta) }),
	"train.tree_method":          strPartial("train.tree_method", func(r *xgboostFiller) *string { return &(r.TreeMethod) }),
	"train.eval_metric":          strPartial("train.eval_metric", func(r *xgboostFiller) *string { return &(r.EvalMetric) }),
	"train.subsample":            fp32Partial("train.subsample", func(r *xgboostFiller) *float32 { return &(r.Subsample) }),
	"train.colsample_bytree":     fp32Partial("train.colsample_bytree", func(r *xgboostFiller) *float32 { return &(r.ColSampleByTree) }),
	"train.colsample_bylevel":    fp32Partial("train.colsample_bylevel", func(r *xgboostFiller) *float32 { return &(r.ColSampleByLevel) }),
	"train.max_bin":              uIntPartial("train.max_bin", func(r *xgboostFiller) *uint { return &(r.MaxBin) }),
	"train.convergence_criteria": strPartial("train.convergence_criteria", func(r *xgboostFiller) *string { return &(r.ConvergenceCriteria) }),
	"train.verbosity":            uIntPartial("train.verbosity", func(r *xgboostFiller) *uint { return &(r.Verbosity) }),
	// xgboost train controllers
	"train.num_round":  uIntPartial("train.num_round", func(r *xgboostFiller) *uint { return &(r.NumRound) }),
	"train.auto_train": boolPartial("train.auto_train", func(r *xgboostFiller) *bool { return &(r.AutoTrain) }),
	// Label, Group, Weight and xgFeatureFields are parsed from columnClause
}

var xgbPredAttrSetterMap = map[string]func(*map[string][]string, *xgboostFiller) error{
	// xgboost output columns (for prediction)
	"pred.append_columns":  sListPartial("pred.append_columns", func(r *xgboostFiller) *[]string { return &(r.AppendColumns) }),
	"pred.result_column":   strPartial("pred.result_column", func(r *xgboostFiller) *string { return &(r.ResultColumn) }),
	"pred.prob_column":     strPartial("pred.prob_column", func(r *xgboostFiller) *string { return &(r.ProbColumn) }),
	"pred.detail_column":   strPartial("pred.detail_column", func(r *xgboostFiller) *string { return &(r.DetailColumn) }),
	"pred.encoding_column": strPartial("pred.encoding_column", func(r *xgboostFiller) *string { return &(r.EncodingColumn) }),
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
	case "XGBOOST.ESTIMATOR":
		if len(filler.Objective) == 0 {
			return xgParseEstimatorError(pr.estimator, fmt.Errorf("objective must be defined"))
		}
	case "XGBOOST.CLASSIFIER":
		if obj := filler.Objective; len(obj) == 0 {
			filler.Objective = "binary:logistic"
		} else if !strings.HasPrefix(obj, "binary") && !strings.HasPrefix(obj, "multi") {
			return xgParseEstimatorError(pr.estimator, fmt.Errorf("found non classification objective(%s)", obj))
		}
	case "XGBOOST.BINARYCLASSIFIER":
		if obj := filler.Objective; len(obj) == 0 {
			filler.Objective = "binary:logistic"
		} else if !strings.HasPrefix(obj, "binary") {
			return xgParseEstimatorError(pr.estimator, fmt.Errorf("found non binary objective(%s)", obj))
		}
	case "XGBOOST.MULTICLASSIFIER":
		if obj := filler.Objective; len(obj) == 0 {
			filler.Objective = "multi:softmax"
		} else if !strings.HasPrefix(obj, "multi") {
			return xgParseEstimatorError(pr.estimator, fmt.Errorf("found non multi-class objective(%s)", obj))
		}
	case "XGBOOST.REGRESSOR":
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

// TODO(sperlingxx): support trainAndValDataset
func newXGBoostFiller(pr *extendedSelect, fts fieldTypes, db *DB) (*xgboostFiller, error) {
	filler := &xgboostFiller{
		ModelPath: pr.save,
	}
	filler.IsTrain = pr.train
	filler.StandardSelect = pr.standardSelect.String()

	// solve keyword: WITH (attributes)
	if e := xgParseAttr(pr, filler); e != nil {
		return nil, fmt.Errorf("failed to set xgboost attributes: %v", e)
	}
	// set default value of result column field in pred mode
	if !pr.train && len(filler.ResultColumn) == 0 {
		filler.ResultColumn = "result"
	}

	if pr.train {
		// solve keyword: TRAIN (estimator）
		if e := xgParseEstimator(pr, filler); e != nil {
			return nil, e
		}
	} else {
		// solve keyword: PREDICT (output_table）
		if len(pr.into) == 0 {
			return nil, fmt.Errorf("missing output table in xgboost prediction clause")
		}
		filler.OutputTable = pr.into
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
	jsonBuffer, e := json.Marshal(filler.xgLearningFields)
	if e != nil {
		return nil, e
	}
	filler.LearningJSON = string(jsonBuffer)

	jsonBuffer, e = json.Marshal(filler.xgDataSourceFields)
	if e != nil {
		return nil, e
	}
	filler.DataSourceJSON = string(jsonBuffer)

	jsonBuffer, e = json.Marshal(filler.xgColumnFields)
	if e != nil {
		return nil, e
	}
	filler.ColumnJSON = string(jsonBuffer)

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

func xgCreatePredictionTable(pr *extendedSelect, r *xgboostFiller, db *DB) error {
	dropStmt := fmt.Sprintf("drop table if exists %s;", r.OutputTable)
	if _, e := db.Exec(dropStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", dropStmt, e)
	}

	fts, e := verify(pr, db)
	if e != nil {
		return e
	}

	var b bytes.Buffer
	fmt.Fprintf(&b, "create table %s (", r.OutputTable)
	for _, col := range r.AppendColumns {
		typ, ok := fts.get(col)
		if !ok {
			return fmt.Errorf("xgCreatePredictionTable: Cannot find type of field %s", col)
		}
		stype, e := universalizeColumnType(db.driverName, typ)
		if e != nil {
			return e
		}
		fmt.Fprintf(&b, "%s %s, ", col, stype)
	}
	// add prob column
	if len(r.ProbColumn) > 0 {
		stype, e := universalizeColumnType(db.driverName, "DOUBLE")
		if e != nil {
			return e
		}
		fmt.Fprintf(&b, "%s %s, ", r.ProbColumn, stype)
	}
	// add detail column
	if len(r.DetailColumn) > 0 {
		stype, e := universalizeColumnType(db.driverName, "VARCHAR")
		if e != nil {
			return e
		}
		fmt.Fprintf(&b, "%s %s, ", r.DetailColumn, stype)
	}
	// add encoding column
	if len(r.EncodingColumn) > 0 {
		stype, e := universalizeColumnType(db.driverName, "VARCHAR")
		if e != nil {
			return e
		}
		fmt.Fprintf(&b, "%s %s, ", r.EncodingColumn, stype)
	}
	// add result column
	stype, e := universalizeColumnType(db.driverName, "DOUBLE")
	if e != nil {
		return e
	}
	fmt.Fprintf(&b, "%s %s);", r.ResultColumn, stype)

	createStmt := b.String()
	if _, e := db.Exec(createStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", createStmt, e)
	}
	return nil
}

var xgTemplate = template.Must(template.New("codegenXG").Parse(xgTemplateText))

const xgTemplateText = `
from launcher.config_fields import JobType
from sqlflow_submitter.xgboost import run_with_sqlflow

{{if .IsTrain}}
mode = JobType.TRAIN
{{else}}
mode = JobType.PREDICT
{{end}}

run_with_sqlflow(
	mode=mode,
	model_path='{{.ModelPath}}',
	learning_config='{{.LearningJSON}}',	
	data_source_config='{{.DataSourceJSON}}',	
	column_config='{{.ColumnJSON}}')
{{if .IsTrain}}
print("Done training.")
{{else}}
print("Done prediction, the result table: {{.OutputTable}}")
{{end}}
`
