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
	"io"
	"os"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-sql-driver/mysql"
	"sqlflow.org/gohive"
	"sqlflow.org/gomaxcompute"
)

type antXGBoostFiller struct {
	ModelPath string
	xgLearningFields
	xgColumnFields
	xgDataSourceFields
	validDataSource     xgDataSourceFields
	LearningJSON        string
	DataSourceJSON      string
	ValidDataSourceJSON string
	ColumnJSON          string
}

type xgLearningFields struct {
	NumRound        uint `json:"num_boost_round,omitempty"`
	AutoTrain       bool `json:"auto_train"`
	xgBoosterFields `json:"params,omitempty"`
}

type xgBoosterFields struct {
	Objective        string  `json:"objective,omitempty"`
	EvalMetric       string  `json:"eval_metric,omitempty"`
	Booster          string  `json:"booster,omitempty"`
	Seed             uint    `json:"seed,omitempty"`
	NumClass         uint    `json:"num_class,omitempty"`
	Eta              float32 `json:"eta,omitempty"`
	Gamma            float32 `json:"gamma,omitempy"`
	MaxDepth         uint    `json:"max_depth,omitempty"`
	MinChildWeight   uint    `json:"min_child_weight,omitempty"`
	Subsample        float32 `json:"subsample,omtiempty"`
	ColSampleByTree  float32 `json:"colsample_bytree,omitempty"`
	ColSampleByLevel float32 `json:"colsample_bylevel,omitempty"`
	ColSampleByNode  float32 `json:"colsample_bynode,omitempty"`
	// `Lambda` is reversed in python, so we use alias reg_lambda.
	Lambda float32 `json:"reg_lambda,omitempty"`
	// We use alias `reg_alpha` to keep align with `reg_lambda`。
	Alpha               float32 `json:"reg_alpha,omitempty"`
	TreeMethod          string  `json:"tree_method,omitempty"`
	SketchEps           float32 `json:"sketch_eps,omitempty"`
	ScalePosWeight      float32 `json:"scale_pos_weight,omitempty"`
	GrowPolicy          string  `json:"grow_policy,omitempty"`
	MaxLeaves           uint    `json:"max_leaves,omitempty"`
	MaxBin              uint    `json:"max_bin,omitempty"`
	NumParallelTree     uint    `json:"num_parallel_tree,omitempty"`
	ConvergenceCriteria string  `json:"convergence_criteria,omitempty"` // auto_train config
	Verbosity           uint    `json:"verbosity,omitempty"`            // auto_train config
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
	ResultColumn   string `json:"result_column"`
	ProbColumn     string `json:"probability_column"`
	DetailColumn   string `json:"detail_column"`
	EncodingColumn string `json:"leaf_column"`
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
	xgDataBaseHiveField
}

type xgDataBaseHiveField struct {
	HiveAuth    string            `json:"auth,omitempty"`
	HiveSession map[string]string `json:"session,omitempty"`
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
	return fmt.Errorf("xgUnknownFCError: feature column keyword(`%s`) is not supported by ant-xgboost engine", kw)
}

func xgUnsupportedColTagError() error {
	return fmt.Errorf("xgUnsupportedColTagError: valid column tags of ant-xgboost engine([feature_columns, group, weight])")
}

func uIntPartial(key string, ptrFn func(*antXGBoostFiller) *uint) func(*map[string][]string, *antXGBoostFiller) error {
	return func(a *map[string][]string, r *antXGBoostFiller) error {
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

func fp32Partial(key string, ptrFn func(*antXGBoostFiller) *float32) func(*map[string][]string, *antXGBoostFiller) error {
	return func(a *map[string][]string, r *antXGBoostFiller) error {
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

func boolPartial(key string, ptrFn func(*antXGBoostFiller) *bool) func(*map[string][]string, *antXGBoostFiller) error {
	return func(a *map[string][]string, r *antXGBoostFiller) error {
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

func strPartial(key string, ptrFn func(*antXGBoostFiller) *string) func(*map[string][]string, *antXGBoostFiller) error {
	return func(a *map[string][]string, r *antXGBoostFiller) error {
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

func sListPartial(key string, ptrFn func(*antXGBoostFiller) *[]string) func(*map[string][]string, *antXGBoostFiller) error {
	return func(a *map[string][]string, r *antXGBoostFiller) error {
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

var xgbTrainAttrSetterMap = map[string]func(*map[string][]string, *antXGBoostFiller) error{
	// booster params
	"train.objective":            strPartial("train.objective", func(r *antXGBoostFiller) *string { return &(r.Objective) }),
	"train.eval_metric":          strPartial("train.eval_metric", func(r *antXGBoostFiller) *string { return &(r.EvalMetric) }),
	"train.booster":              strPartial("train.booster", func(r *antXGBoostFiller) *string { return &(r.Booster) }),
	"train.seed":                 uIntPartial("train.seed", func(r *antXGBoostFiller) *uint { return &(r.Seed) }),
	"train.num_class":            uIntPartial("train.num_class", func(r *antXGBoostFiller) *uint { return &(r.NumClass) }),
	"train.eta":                  fp32Partial("train.eta", func(r *antXGBoostFiller) *float32 { return &(r.Eta) }),
	"train.gamma":                fp32Partial("train.gamma", func(r *antXGBoostFiller) *float32 { return &(r.Gamma) }),
	"train.max_depth":            uIntPartial("train.max_depth", func(r *antXGBoostFiller) *uint { return &(r.MaxDepth) }),
	"train.min_child_weight":     uIntPartial("train.min_child_weight", func(r *antXGBoostFiller) *uint { return &(r.MinChildWeight) }),
	"train.subsample":            fp32Partial("train.subsample", func(r *antXGBoostFiller) *float32 { return &(r.Subsample) }),
	"train.colsample_bytree":     fp32Partial("train.colsample_bytree", func(r *antXGBoostFiller) *float32 { return &(r.ColSampleByTree) }),
	"train.colsample_bylevel":    fp32Partial("train.colsample_bylevel", func(r *antXGBoostFiller) *float32 { return &(r.ColSampleByLevel) }),
	"train.colsample_bynode":     fp32Partial("train.colsample_bynode", func(r *antXGBoostFiller) *float32 { return &(r.ColSampleByNode) }),
	"train.lambda":               fp32Partial("train.lambda", func(r *antXGBoostFiller) *float32 { return &(r.Lambda) }),
	"train.alpha":                fp32Partial("train.alpha", func(r *antXGBoostFiller) *float32 { return &(r.Alpha) }),
	"train.tree_method":          strPartial("train.tree_method", func(r *antXGBoostFiller) *string { return &(r.TreeMethod) }),
	"train.sketch_eps":           fp32Partial("train.sketch_eps", func(r *antXGBoostFiller) *float32 { return &(r.SketchEps) }),
	"train.scale_pos_weight":     fp32Partial("train.scale_pos_weight", func(r *antXGBoostFiller) *float32 { return &(r.ScalePosWeight) }),
	"train.grow_policy":          strPartial("train.grow_policy", func(r *antXGBoostFiller) *string { return &(r.GrowPolicy) }),
	"train.max_leaves":           uIntPartial("train.max_leaves", func(r *antXGBoostFiller) *uint { return &(r.MaxLeaves) }),
	"train.max_bin":              uIntPartial("train.max_bin", func(r *antXGBoostFiller) *uint { return &(r.MaxBin) }),
	"train.num_parallel_tree":    uIntPartial("train.num_parallel_tree", func(r *antXGBoostFiller) *uint { return &(r.NumParallelTree) }),
	"train.convergence_criteria": strPartial("train.convergence_criteria", func(r *antXGBoostFiller) *string { return &(r.ConvergenceCriteria) }),
	"train.verbosity":            uIntPartial("train.verbosity", func(r *antXGBoostFiller) *uint { return &(r.Verbosity) }),
	// xgboost train controllers
	"train.num_round":  uIntPartial("train.num_round", func(r *antXGBoostFiller) *uint { return &(r.NumRound) }),
	"train.auto_train": boolPartial("train.auto_train", func(r *antXGBoostFiller) *bool { return &(r.AutoTrain) }),
	// Label, Group, Weight and xgFeatureFields are parsed from columnClause
}

var xgbPredAttrSetterMap = map[string]func(*map[string][]string, *antXGBoostFiller) error{
	// xgboost output columns (for prediction)
	"pred.append_columns":  sListPartial("pred.append_columns", func(r *antXGBoostFiller) *[]string { return &(r.AppendColumns) }),
	"pred.prob_column":     strPartial("pred.prob_column", func(r *antXGBoostFiller) *string { return &(r.ProbColumn) }),
	"pred.detail_column":   strPartial("pred.detail_column", func(r *antXGBoostFiller) *string { return &(r.DetailColumn) }),
	"pred.encoding_column": strPartial("pred.encoding_column", func(r *antXGBoostFiller) *string { return &(r.EncodingColumn) }),
	// Label, Group, Weight and xgFeatureFields are parsed from columnClause
}

func xgParseAttr(pr *extendedSelect, r *antXGBoostFiller) error {
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

	// fill antXGBoostFiller with attrs
	var setterMap map[string]func(*map[string][]string, *antXGBoostFiller) error
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
func parseFeatureColumns(columns *exprlist, r *antXGBoostFiller) error {
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
func parseSparseKeyValueFeatures(colSpecs []*columnSpec, r *antXGBoostFiller) error {
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

func parseDenseFeatures(feaCols []featureColumn, r *antXGBoostFiller) error {
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

func xgParseColumns(pr *extendedSelect, filler *antXGBoostFiller) error {
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
	if pr.train || pr.analyze {
		filler.LabelField = &xgFeatureMeta{
			FeatureName: pr.label,
		}
		filler.Label = pr.label
	}

	return nil
}

func xgParseEstimator(pr *extendedSelect, filler *antXGBoostFiller) error {
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
			filler.Objective = "multi:softprob"
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

func newAntXGBoostFiller(pr *extendedSelect, ds *trainAndValDataset, db *DB) (*antXGBoostFiller, error) {
	filler := &antXGBoostFiller{
		ModelPath: pr.save,
	}
	filler.IsTrain = pr.train
	filler.StandardSelect = pr.standardSelect.String()

	// solve keyword: WITH (attributes)
	if e := xgParseAttr(pr, filler); e != nil {
		return nil, fmt.Errorf("failed to set xgboost attributes: %v", e)
	}

	if pr.train {
		// solve keyword: TRAIN (estimator）
		if e := xgParseEstimator(pr, filler); e != nil {
			return nil, e
		}
	} else if !pr.analyze {
		// solve keyword: PREDICT (output_table.result_column）
		var e error
		filler.OutputTable, filler.ResultColumn, e = parseTableColumn(pr.into)
		if e != nil {
			return nil, fmt.Errorf("invalid predParsed.into, %v", e)
		}
	}

	if !pr.train {
		// remove detail & prob column field when non-classification objective found
		classObj := filler.Objective == "binary:logistic" || filler.Objective == "multi:softprob"
		if len(filler.DetailColumn) > 0 && !classObj {
			filler.DetailColumn = ""
		}
		if len(filler.ProbColumn) > 0 && !classObj {
			filler.ProbColumn = ""
		}
	}

	// solve keyword: COLUMN (column clauses)
	if e := xgParseColumns(pr, filler); e != nil {
		return nil, e
	}

	// rewrite (train & valid) data source if ds is defined
	if pr.train && ds != nil {
		filler.validDataSource = filler.xgDataSourceFields
		filler.StandardSelect = fmt.Sprintf("SELECT * FROM %s", ds.training)
		filler.validDataSource.StandardSelect = fmt.Sprintf("SELECT * FROM %s", ds.validation)
	}

	// fill data base info
	if e := xgFillDatabaseInfo(&filler.xgDataSourceFields, db); e != nil {
		return nil, e
	}
	if len(filler.validDataSource.StandardSelect) > 0 {
		if e := xgFillDatabaseInfo(&filler.validDataSource, db); e != nil {
			return nil, e
		}
	}

	// serialize fields
	jsonBuffer, e := json.Marshal(filler.xgLearningFields)
	if e != nil {
		return nil, e
	}
	filler.LearningJSON = string(jsonBuffer)

	jsonBuffer, e = json.Marshal(filler.xgColumnFields)
	if e != nil {
		return nil, e
	}
	filler.ColumnJSON = string(jsonBuffer)

	jsonBuffer, e = json.Marshal(filler.xgDataSourceFields)
	if e != nil {
		return nil, e
	}
	filler.DataSourceJSON = string(jsonBuffer)

	if len(filler.validDataSource.StandardSelect) > 0 {
		jsonBuffer, e = json.Marshal(filler.validDataSource)
		if e != nil {
			return nil, e
		}
		filler.ValidDataSourceJSON = string(jsonBuffer)
	}

	return filler, nil
}

func xgFillDatabaseInfo(r *xgDataSourceFields, db *DB) error {
	r.Driver = db.driverName
	switch db.driverName {
	case "mysql":
		cfg, err := mysql.ParseDSN(db.dataSourceName)
		if err != nil {
			return err
		}
		sa := strings.Split(cfg.Addr, ":")
		r.Host, r.Port, r.Database = sa[0], sa[1], cfg.DBName
		r.User, r.Password = cfg.User, cfg.Passwd
	case "sqlite3":
		r.Database = db.dataSourceName
	case "hive":
		cfg, err := gohive.ParseDSN(db.dataSourceName)
		if err != nil {
			return err
		}
		r.HiveAuth = cfg.Auth
		if len(cfg.SessionCfg) > 0 {
			r.HiveSession = cfg.SessionCfg
		}
		sa := strings.Split(cfg.Addr, ":")
		r.Host, r.Port, r.Database = sa[0], sa[1], cfg.DBName
		r.User, r.Password = cfg.User, cfg.Passwd
		// remove the last ';' which leads to a ParseException
		r.StandardSelect = strings.TrimSuffix(r.StandardSelect, ";")
	case "maxcompute":
		cfg, err := gomaxcompute.ParseDSN(db.dataSourceName)
		if err != nil {
			return err
		}
		// setting r.Port=0 just makes connect() happy
		r.Host, r.Port, r.Database = cfg.Endpoint, "0", cfg.Project
		r.User, r.Password = cfg.AccessID, cfg.AccessKey
	default:
		return fmt.Errorf("sqlfow currently doesn't support DB %v", db.driverName)
	}
	return nil
}

func xgCreatePredictionTable(pr *extendedSelect, r *antXGBoostFiller, db *DB) error {
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
	if db.driverName == "hive" {
		hdfsPath := os.Getenv("SQLFLOW_HIVE_LOCATION_ROOT_PATH")
		if hdfsPath == "" {
			hdfsPath = "/sqlflow"
		}
		fmt.Fprintf(&b, "%s %s) ROW FORMAT DELIMITED FIELDS TERMINATED BY \"\\001\" STORED AS TEXTFILE LOCATION \"%s/%s\" ;", r.ResultColumn, stype, hdfsPath, r.OutputTable)
	} else {
		fmt.Fprintf(&b, "%s %s);", r.ResultColumn, stype)
	}

	createStmt := b.String()
	if _, e := db.Exec(createStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", createStmt, e)
	}
	return nil
}

func genXG(w io.Writer, pr *extendedSelect, ds *trainAndValDataset, fts fieldTypes, db *DB) error {
	r, e := newAntXGBoostFiller(pr, ds, db)
	if e != nil {
		return e
	}
	if pr.train {
		return xgTemplate.Execute(w, r)
	}
	if e := xgCreatePredictionTable(pr, r, db); e != nil {
		return fmt.Errorf("failed to create prediction table: %v", e)
	}
	return xgTemplate.Execute(w, r)
}

var xgTemplate = template.Must(template.New("codegenXG").Parse(xgTemplateText))

const xgTemplateText = `
from launcher.config_fields import JobType
from sqlflow_submitter.ant_xgboost import run_with_sqlflow

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
	valid_data_source_config='{{.ValidDataSourceJSON}}',
	column_config='{{.ColumnJSON}}')
{{if .IsTrain}}
print("Done training.")
{{else}}
print("Done prediction, the result table: {{.OutputTable}}")
{{end}}
`
