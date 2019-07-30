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
	"sqlflow.org/gohive"
	"sqlflow.org/gomaxcompute"
	"strconv"
	"strings"
)

type xgboostFiller struct {
	isTrain bool
	standardSelect string
	modelPath string
	xgboostFields
	xgColumnFields
	xgDataSourceFields
	xgRuntimeFields
}

type xgRuntimeFields struct {
	runLocal bool
	xgRuntimeResourceFields
}

type xgRuntimeResourceFields struct {
	WorkerNum  uint `json:"worker_num,omitempty"`
	MemorySize uint `json:"memory_size,omitempty"`
	CPUSize    uint `json:"cpu_size,omitempty"`
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
	IsTensorFlowIntegrated bool             `json:"is_tf_integrated,omitempty"`
	X                      []*xgFeatureMeta `json:"x,omitempty"`
	LabelField             *xgFeatureMeta   `json:"label,omitempty"`
	WeightField            *xgFeatureMeta   `json:"weight,omitempty"`
	GroupField             *xgFeatureMeta   `json:"group,omitempty"`
	xgDataBaseField        `json:"db_config,omitempty"`
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

func uIntPartial(key string, ptrFn func(*xgboostFiller) *uint) func(*map[string][]string, *xgboostFiller) error {
	return func(a *map[string][]string, r *xgboostFiller) error {
		// setXGBoostAttr will ensure the key is existing in map
		val, _ := (*a)[key]
		if len(val) != 1 {
			return fmt.Errorf("invalid attr value(%v) for key(%s)", val, key)
		}
		if intVal, err := strconv.ParseUint(val[0], 10, 32); err != nil {
			return err
		} else if intPtr := ptrFn(r); *intPtr != 0 {
			return fmt.Errorf("duplicate xgboost (int)attr setting, the key of attr is %s", key)
		} else {
			*intPtr = uint(intVal)
			delete(*a, key)
		}
		return nil
	}
}

func fp32Partial(key string, ptrFn func(*xgboostFiller) *float32) func(*map[string][]string, *xgboostFiller) error {
	return func(a *map[string][]string, r *xgboostFiller) error {
		// setXGBoostAttr will ensure the key is existing in map
		val, _ := (*a)[key]
		if len(val) != 1 {
			return fmt.Errorf("invalid attr value(%v) for key(%s)", val, key)
		}
		if fpVal, err := strconv.ParseFloat(val[0], 32); err != nil {
			return err
		} else if fpPtr := ptrFn(r); *fpPtr != 0 {
			return fmt.Errorf("duplicate xgboost (float)attr setting, the key of attr is %s", key)
		} else {
			*fpPtr = float32(fpVal)
			delete(*a, key)
		}
		return nil
	}
}

func boolPartial(key string, ptrFn func(*xgboostFiller) *bool) func(*map[string][]string, *xgboostFiller) error {
	return func(a *map[string][]string, r *xgboostFiller) error {
		// setXGBoostAttr will ensure the key is existing in map
		val, _ := (*a)[key]
		if len(val) != 1 {
			return fmt.Errorf("invalid attr value(%v) for key(%s)", val, key)
		}
		bVal, err := strconv.ParseBool(val[0])
		if err != nil {
			return err
		}
		bPtr := ptrFn(r)
		*bPtr = bVal
		delete(*a, key)
		return nil
	}
}

func strPartial(key string, ptrFn func(*xgboostFiller) *string) func(*map[string][]string, *xgboostFiller) error {
	return func(a *map[string][]string, r *xgboostFiller) error {
		// setXGBoostAttr will ensure the key is existing in map
		val, _ := (*a)[key]
		if len(val) != 1 {
			return fmt.Errorf("invalid attr value(%v) for key(%s)", val, key)
		}
		stringPtr := ptrFn(r)
		if len(*stringPtr) != 0 {
			return fmt.Errorf("duplicate xgboost (string)attr setting, the key of attr is %s", key)
		}
		*stringPtr = val[0]
		delete(*a, key)
		return nil
	}
}

func sListPartial(key string, ptrFn func(*xgboostFiller) []string) func(*map[string][]string, *xgboostFiller) error {
	return func(a *map[string][]string, r *xgboostFiller) error {
		// setXGBoostAttr will ensure the key is existing in map
		val, _ := (*a)[key]
		strListPtr := ptrFn(r)
		if len(strListPtr) != 0 {
			return fmt.Errorf("duplicate xgboost (string list)attr setting, the key of attr is %s", key)
		}
		strListPtr = val
		delete(*a, key)
		return nil
	}
}

var xgbAttrSetterMap = map[string]func(*map[string][]string, *xgboostFiller) error{
	// runtime params
	"run_local": boolPartial("run_local", func(r *xgboostFiller) *bool { return &(r.runLocal) }),
	"workers":   uIntPartial("workers", func(r *xgboostFiller) *uint { return &(r.WorkerNum) }),
	"memory":    uIntPartial("memory", func(r *xgboostFiller) *uint { return &(r.MemorySize) }),
	"cpu":       uIntPartial("cpu", func(r *xgboostFiller) *uint { return &(r.CPUSize) }),
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
	// xgboost output columns (for prediction)
	"append_columns":  sListPartial("append_columns", func(r *xgboostFiller) []string { return r.AppendColumns }),
	"result_column":   strPartial("result_column", func(r *xgboostFiller) *string { return &(r.ResultColumn) }),
	"prob_column":     strPartial("prob_column", func(r *xgboostFiller) *string { return &(r.ProbColumn) }),
	"detail_column":   strPartial("detail_column", func(r *xgboostFiller) *string { return &(r.DetailColumn) }),
	"encoding_column": strPartial("encoding_column", func(r *xgboostFiller) *string { return &(r.EncodingColumn) }),
	// Label, Group, Weight and xgFeatureFields are parsed from columnClause
}

func setXGBoostAttr(attrs *map[string][]string, r *xgboostFiller) error {
	for k := range *attrs {
		if setter, ok := xgbAttrSetterMap[k]; ok {
			if e := setter(attrs, r); e != nil {
				return e
			}
		}
	}
	return nil
}

/* parse feature column, which owned by default column target("feature_columns:), from AST(pr.columns)

   	For now, two schemas are supported:
		1. sparse-kv
			schema: COLUMN SPARSE([feature_column], [1-dim shape], [single char delimiter])
			data example: COLUMN SPARSE("0:1.5 1:100.1f 11:-1.2", [20], " ")
		2. tf feature columns
			 roughly same as TFEstimator, except output shape of feaColumns are required to be 1-dim.
 */
func parseFeatureColumns(columns *exprlist, r *xgboostFiller) error {
	feaCols, colSpecs, err := resolveTrainColumns(columns)
	if err != nil {
		return err
	}
	r.IsTensorFlowIntegrated = true
	if len(colSpecs) != 0 {
		if len(feaCols) != 0 {
			return fmt.Errorf("if SPARSE column is defined, there shouldn't exist another feature columns")
		}
		return parseSparseKeyValueFeatures(colSpecs, r)
	}
	if e := parseDenseFeatures(feaCols, r); e != nil {
		return e
	}

	return nil
}

// parse sparse kv feature, which identified by `SPARSE`.
// ex: SPARSE(col1, [100], ",")
func parseSparseKeyValueFeatures(colSpecs []*columnSpec, r *xgboostFiller) error {
	if len(colSpecs) > 1 {
		return fmt.Errorf("detect more than one SPARSE column, SPARSE column should be unqiue")
	}
	spec := colSpecs[0]
	if !spec.IsSparse {
		return fmt.Errorf("DENSE column is not supported by xgboost engine")
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

	if allSimpleCol {
		for _, fm := range r.X {
			fm.FeatureColumnCode = ""
			r.FeatureColumns = append(r.FeatureColumns, fm.FeatureName)
		}
		r.FeatureSize = uint(len(r.X))
		r.IsTensorFlowIntegrated = false
	}
	r.IsSparse = false

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

func newXGBoostFiller(pr *extendedSelect, fts fieldTypes, db *DB) (*xgboostFiller, error) {
	filler := &xgboostFiller{isTrain: pr.train, modelPath: pr.save}

	xgbAttrs := make(map[string][]string)
	for k, exp := range pr.attrs {
		strExp := exp.String()
		if strings.HasPrefix(strExp, "[") && strings.HasSuffix(strExp, "]") {
			xgbAttrs[k] = exp.cdr()
			continue
		}
		xgbAttrs[k] = []string{strExp}
	}

	if e := setXGBoostAttr(&xgbAttrs, filler); e != nil {
		return nil, fmt.Errorf("failed to set xgboost attributes: %exp", e)
	}
	// remaining elements in xgbAttrs are unsolved ones, so we throw exception if any elements remaining.
	if len(xgbAttrs) > 0 {
		for k, v := range xgbAttrs {
			log.Errorf("unsolved xgboost attr: %s = %s", k, v)
		}
		return nil, fmt.Errorf("found unsolved xgboost attributes")
	}

	// solve keyword: TRAIN (estimator)
	switch strings.ToUpper(pr.estimator) {
	case "XGBOOSTESTIMATOR":
		if len(filler.Objective) == 0 {
			return nil, fmt.Errorf("objective must be defined, when using XGBoostEstimator")
		}
	case "XGBOOSTCLASSIFIER":
		if obj := filler.Objective; len(obj) == 0 {
			filler.Objective = "binary:logistic"
		} else if !strings.HasPrefix(obj, "binary") && !strings.HasPrefix(obj, "multi") {
			return nil, fmt.Errorf("found non classification objective(%s), when using XGBoostClassifier", obj)
		}
	case "XGBOOSTBINARYCLASSIFIER":
		if obj := filler.Objective; len(obj) == 0 {
			filler.Objective = "binary:logistic"
		} else if !strings.HasPrefix(obj, "binary") {
			return nil, fmt.Errorf("found non binary objective(%s), when using XGBoostBinaryClassifier", obj)
		}
	case "XGBOOSTMULTICLASSIFIER":
		if obj := filler.Objective; len(obj) == 0 {
			filler.Objective = "multi:softmax"
		} else if !strings.HasPrefix(obj, "multi") {
			return nil, fmt.Errorf("found non multi-class objective(%s), when using XGBoostMultiClassifier", obj)
		}
	case "XGBOOSTREGRESSOR":
		if obj := filler.Objective; len(obj) == 0 {
			filler.Objective = "reg:squarederror"
		} else if !strings.HasPrefix(obj, "reg") && !strings.HasPrefix(obj, "rank") {
			return nil, fmt.Errorf("found non reg objective(%s), when using XGBoostRegressor", obj)
		}
	default:
		return nil, fmt.Errorf("unknown xgboost estimator: %s", pr.estimator)
	}

	// solve columns
	for target, columns := range pr.columns {
		switch target {
		case "feature_columns":
			if e := parseFeatureColumns(&columns, filler); e != nil {
				return nil, fmt.Errorf("failed to parse feature columns, %v", e)
			}
		case "label":
			colMeta, err := parseSimpleColumn("label", &columns);
			if err != nil {
				return nil, fmt.Errorf("failed to parse LABEL, %v", err)
			}
			filler.LabelField = colMeta
		case "group":
			colMeta, err := parseSimpleColumn("group", &columns)
			if err != nil {
				return nil, fmt.Errorf("failed to parse GROUP, %v", err)
			}
			filler.GroupField = colMeta
		case "weight":
			colMeta, err := parseSimpleColumn("weight", &columns)
			if err != nil {
				return nil, fmt.Errorf("failed to parse WEIGHT, %v", err)
			}
			filler.WeightField = colMeta
		default:
			return nil, fmt.Errorf("unsupported COLUMN TAG: %s", target)
		}
	}

	return xgFillDatabaseInfo(filler, db)
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
		r.standardSelect = removeLastSemicolon(r.standardSelect)
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
