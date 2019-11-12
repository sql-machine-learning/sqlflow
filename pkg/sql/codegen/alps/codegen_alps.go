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

package alps

import (
	// "bytes"
	"fmt"
	"io/ioutil"
	"os"
	// "os/exec"
	"database/sql"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"sqlflow.org/gomaxcompute"
	pb "sqlflow.org/sqlflow/pkg/server/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen"
	"sqlflow.org/sqlflow/pkg/sql/columns"
	sqlflow "sqlflow.org/sqlflow/pkg/sql"
)

var alpsTrainTemplate = template.Must(template.New("alps_train").Parse(alpsTrainTemplateText))
var alpsPredTemplate = template.Must(template.New("alps_predict").Parse(alpsPredTemplateText))


type alpsFillerWithIR struct {
	// Training or Predicting
	IsTraining bool

	// Input & Output
	TrainInputTable    string
	EvalInputTable     string
	PredictInputTable  string
	ModelDir           string
	ScratchDir         string
	PredictOutputTable string
	PredictInputModel  string

	// Schema & Decode info
	Fields string
	X      string
	Y      string

	// Train
	ImportCode        string
	RemoteModuleCode  string
	ModelCreatorCode  string
	FeatureColumnCode string
	TrainClause       *resolvedTrainClauseWithIR
	ExitOnSubmit      bool

	// Predict
	PredictUDF string

	// Feature map
	FeatureMapTable     string
	FeatureMapPartition string

	// ODPS
	OdpsConf   *gomaxcompute.Config
	EngineCode string

	// OSS Credential
	UserID      string
	OSSID       string
	OSSKey      string
	OSSEndpoint string
}

func engineCreatorCodeWithIR(resolved *resolvedTrainClauseWithIR) (string, error) {
	if resolved.EngineParams.etype == "local" {
		return "LocalEngine()", nil
	}
	engine := resolved.EngineParams

	var engineName string
	if engine.etype == "k8s" {
		engineName = "KubemakerEngine"
	} else if engine.etype == "yarn" {
		engineName = "YarnEngine"
	} else {
		return "", fmt.Errorf("Unknown etype %s", engine.etype)
	}

	return fmt.Sprintf("%s(cluster = \"%s\", queue = \"%s\", ps = ResourceConf(memory=%d, num=%d), worker=ResourceConf(memory=%d, num=%d))",
		engineName,
		engine.cluster,
		engine.queue,
		engine.ps.Memory,
		engine.ps.Num,
		engine.worker.Memory,
		engine.worker.Num), nil
}

func modelCreatorCodeWithIR(resolved *resolvedTrainClauseWithIR, args []string) (string, string, string, error) {
	cl := make([]string, 0)
	for key, value := range resolved.ModelConstructorParams {
		
		code, err := generateCodeWithIR(key,value)
		if err != nil {
			return "", "", "", err
		}
		cl = append(cl, code)
	}
	if args != nil {
		for _, arg := range args {
			cl = append(cl, arg)
		}
	}
	modelName := resolved.ModelName
	var importLib string
	if resolved.IsPreMadeModel {
		modelName = fmt.Sprintf("tf.estimator.%s", resolved.ModelName)
	} else {
		parts := strings.Split(modelName, ".")
		importLib = strings.Join(parts[0:len(parts)-1], ".")
	}
	var importCode = ""
	if importLib != "" {
		importCode = fmt.Sprintf("import %s", importLib)
	}

	var remoteModuleCode = ""
	if resolved.CustomModule != nil {
		sha, token, sourceRoot, server := "None", "None", "None", "None"
		customModule := resolved.CustomModule
		if customModule.Sha != "" {
			sha = fmt.Sprintf("\"%s\"", customModule.Sha)
		}
		if customModule.PrivateToken != "" {
			token = fmt.Sprintf("\"%s\"", customModule.PrivateToken)
		}
		if customModule.SourceRoot != "" {
			sourceRoot = fmt.Sprintf("\"%s\"", customModule.SourceRoot)
		}
		if customModule.GitLabServer != "" {
			server = fmt.Sprintf("\"%s\"", customModule.GitLabServer)
		}
		remoteModuleCode = fmt.Sprintf("RemoteModule.create_module(module_name=None, project_name=\"%s\", sha=%s, private_token=%s, source_root=%s, gitlab_server=%s)()",
			customModule.ProjectName, sha, token, sourceRoot, server)
	}

	return remoteModuleCode, importCode,
		fmt.Sprintf("%s(%s)", modelName, strings.Join(cl, ",")), nil
}

func generateCodeWithIR(key string,value interface{}) (string, error) {
	value = attrToPythonValue(value)
	return fmt.Sprintf("%s=%s", key, value), nil
}

func newALPSTrainFillerWithIR(pr *codegen.TrainIR, db *DB, session *pb.Session) (*alpsFillerWithIR, error) {
	label := pr.Label.GetFieldMeta()[0]
	resolved, err := resolveTrainClauseWithIR(pr)
	fmt.Printf("resolved:%+v\n", resolved)
	if err != nil {
		return nil, err
	}

	var odpsConfig = &gomaxcompute.Config{}
	var columnInfo map[string]*columns.ColumnSpec

	// TODO(joyyoj) read feature mapping table's name from table attributes.
	// TODO(joyyoj) pr may contains partition.
	tableName,err := getTableName(pr.Select)
	if err != nil {
		return nil, err
	}
	valTableName,err := getTableName(pr.ValidationSelect)
	if err != nil {
		return nil, err
	}
	var fieldMap = make(map[string]string)
	for _,columnSpecs := range resolved.ColumnSpecs {
		for _,columnSpec := range columnSpecs {
			fieldMap[columnSpec.ColumnName] = columnSpec.ColumnName
		}
	}
	fieldNames := []string{}
	for k := range fieldMap {
		fieldNames = append(fieldNames, k)
	}
	fmap := columns.FeatureMap{
		Table:     tableName + "_feature_map",
		Partition: ""}
	var meta metadata
	fields := make([]string, 0)
	if db != nil {
		odpsConfig, err = gomaxcompute.ParseDSN(db.dataSourceName)
		if err != nil {
			return nil, err
		}
		meta = metadata{odpsConfig, tableName, &fmap, nil}
		fields, err = getFieldsWithIR(&meta, fieldNames, label.Name)
		if err != nil {
			return nil, err
		}
		columnInfo, err = meta.getColumnInfoWithIR(resolved, fields)
		meta.columnInfo = &columnInfo
	} else {
		meta = metadata{odpsConfig, tableName, nil, nil}
		columnInfo = map[string]*columns.ColumnSpec{}
		for _, css := range resolved.ColumnSpecs {
			for _, cs := range css {
				columnInfo[cs.ColumnName] = cs
			}
		}
		meta.columnInfo = &columnInfo
	}
	csCode := make([]string, 0)

	if err != nil {
		log.Fatalf("failed to get column info: %v", err)
		return nil, err
	}

	for _, cs := range columnInfo {
		csCode = append(csCode, cs.ToString())
	}
	y := &columns.ColumnSpec{
		ColumnName: label.Name,
		IsSparse:   false,
		Shape:      []int{1},
		DType:      "int",
		Delimiter:  ","}
	args := make([]string, 0)
	args = append(args, "config=run_config")
	hasFeatureColumns := false
	for _, columns := range resolved.FeatureColumns {
		if len(columns) > 0 {
			hasFeatureColumns = true
		}
	}
	if hasFeatureColumns {
		args = append(args, "feature_columns=feature_columns")
	}
	featureColumnCode := make([]string, 0)
	for _, fcs := range resolved.FeatureColumns {
		codes, err := generateAlpsFeatureColumnCodeWithIR(fcs, &meta)
		if err != nil {
			return nil, err
		}
		for _, code := range codes {
			pycode := fmt.Sprintf("feature_columns.append(%s)", code)
			featureColumnCode = append(featureColumnCode, pycode)
		}
	}
	fcCode := strings.Join(featureColumnCode, "\n        ")
	remoteModuleCode, importCode, modelCode, err := modelCreatorCodeWithIR(resolved, args)
	if err != nil {
		return nil, err
	}
	var engineCode string
	engineCode, err = engineCreatorCodeWithIR(resolved)
	if err != nil {
		return nil, err
	}
	var modelDir string
	var scratchDir string
	exitOnSubmit := true
	userID := ""
	if session != nil {
		exitOnSubmit = session.ExitOnSubmit
		userID = session.UserId
	}
	if resolved.EngineParams.etype == "local" {
		//TODO(uuleon): the scratchDir will be deleted after model uploading
		scratchDir, err = ioutil.TempDir("/tmp", "alps_scratch_dir_")
		if err != nil {
			return nil, err
		}
		modelDir = fmt.Sprintf("%s/model/", scratchDir)
	} else {
		scratchDir = ""
		// TODO(joyyoj) hard code currently.
		modelDir = fmt.Sprintf("arks://%s/%s.tar.gz", filepath.Join("sqlflow", userID), pr.Save)
	}
	var trainInput, evalInput string
	trainInput, evalInput = tableName, valTableName

	log.Printf("Will save the models on: %s\n", modelDir)
	return &alpsFillerWithIR{
		IsTraining:          true,
		TrainInputTable:     trainInput,
		EvalInputTable:      evalInput,
		ScratchDir:          scratchDir,
		ModelDir:            modelDir,
		UserID:              userID,
		Fields:              fmt.Sprintf("[%s]", strings.Join(fields, ",")),
		X:                   fmt.Sprintf("[%s]", strings.Join(csCode, ",")),
		Y:                   y.ToString(),
		OdpsConf:            odpsConfig,
		ImportCode:          importCode,
		RemoteModuleCode:    remoteModuleCode,
		ModelCreatorCode:    modelCode,
		FeatureColumnCode:   fcCode,
		TrainClause:         resolved,
		FeatureMapTable:     fmap.Table,
		FeatureMapPartition: fmap.Partition,
		EngineCode:          engineCode,
		ExitOnSubmit:        exitOnSubmit}, nil
}

func newALPSPredictFillerWithIR(pr *codegen.PredictIR, session *pb.Session) (*alpsFillerWithIR, error) {
	ossID := os.Getenv("OSS_ID")
	ossKey := os.Getenv("OSS_KEY")
	ossEp := os.Getenv("OSS_ENDPOINT")
	if ossID == "" || ossKey == "" || ossEp == "" {
		return nil, fmt.Errorf("Should set env OSS_ID, OSS_KEY and OSS_ENDPOINT while launch sqlflowserver")
	}
	modelDir := fmt.Sprintf("oss://cmps-model/sqlflow/%s/%s.tar.gz", session.UserId, pr.TrainIR.Save)
	valTableName,err := getTableName(pr.Select)
	if err != nil {
		fmt.Printf("getTableName_error, %v \n",pr.Select)
		return nil, err
	}
	return &alpsFillerWithIR{
		IsTraining:         false,
		PredictInputTable:  valTableName,
		PredictOutputTable: pr.ResultTable,
		PredictUDF:         pr.Select,
		ModelDir:           modelDir,
		UserID:             session.UserId,
		OSSID:              ossID,
		OSSKey:             ossKey,
		OSSEndpoint:        ossEp,
	}, nil
}

func getTableName(selectSQL string) (string,error) {
	parser := sqlflow.NewParser()
	r, e := parser.Parse(selectSQL)
	if e != nil {
		return "", e
	}
	tableName := sqlflow.GetTableNames(r)
	return tableName,nil
}

func generateAlpsFeatureColumnCodeWithIR(fcs []codegen.FeatureColumn, metadata *metadata) ([]string, error) {
	var codes = make([]string, 0, 1000)
	for _, fc := range fcs {
		code,err := generateFeatureColumnCode(fc)
		if err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, nil
}

func generateFeatureColumnCode(fc codegen.FeatureColumn) (string, error) {
	switch c := fc.(type) {
	case *codegen.NumericColumn:
		nc := fc.(*codegen.NumericColumn)
		return fmt.Sprintf("tf.feature_column.numeric_column(\"%s\", shape=%s)",
			nc.FieldMeta.Name,
			intArrayToJSONString(nc.FieldMeta.Shape)), nil
	case *codegen.BucketColumn:
		bc := fc.(*codegen.BucketColumn)
		sourceCode, err := generateFeatureColumnCode(bc.SourceColumn)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(
			"tf.feature_column.bucketized_column(%s, boundaries=%s)",
			sourceCode,
			intArrayToJSONString(bc.Boundaries)), nil
	case *codegen.CategoryIDColumn:
		cc := fc.(*codegen.CategoryIDColumn)
		return fmt.Sprintf("tf.feature_column.categorical_column_with_identity(key=\"%s\", num_buckets=%d)",
			cc.FieldMeta.Name, cc.BucketSize), nil
	case *codegen.SeqCategoryIDColumn:
		cc := fc.(*codegen.SeqCategoryIDColumn)
		return fmt.Sprintf("tf.feature_column.sequence_categorical_column_with_identity(key=\"%s\", num_buckets=%d)",
			cc.FieldMeta.Name, cc.BucketSize), nil
	case *codegen.CrossColumn:
		cc := fc.(*codegen.CrossColumn)
		var keysGenerated = make([]string, len(cc.Keys))
		for idx, key := range cc.Keys {
			if c, ok := key.(codegen.FeatureColumn); ok {
				code, err := generateFeatureColumnCode(c)
				if err != nil {
					return "", err
				}
				keysGenerated[idx] = code
			} else {
				return "", fmt.Errorf("field in cross column is not a FeatureColumn type: %v", key)
			}
		}
		return fmt.Sprintf(
			"tf.feature_column.crossed_column([%s], hash_bucket_size=%d)",
			strings.Join(keysGenerated, ","), cc.HashBucketSize), nil
	case *codegen.EmbeddingColumn:
		ec := fc.(*codegen.EmbeddingColumn)
		catColumn, ok := ec.CategoryColumn.(codegen.FeatureColumn)
		if !ok {
			return "", fmt.Errorf("embedding generate code error, input is not featureColumn: %s", ec.CategoryColumn)
		}
		sourceCode, err := generateFeatureColumnCode(catColumn)
		if err != nil {
			return "", err
		}
		initializer := "None"
		if ec.Initializer != "" {
			initializer = ec.Initializer
		}
		return fmt.Sprintf("tf.feature_column.embedding_column(%s, dimension=%d, combiner=\"%s\", initializer=%s)",
			sourceCode, ec.Dimension, ec.Combiner, initializer), nil
	default:
		return "", fmt.Errorf("unsupported feature column type %T on %v", c, c)
	}
}

func intArrayToJSONString(ia []int) string {
	return strings.Join(strings.Split(fmt.Sprint(ia), " "), ",")
}

type metadata struct {
	odpsConfig *gomaxcompute.Config
	table      string
	featureMap *columns.FeatureMap
	columnInfo *map[string]*columns.ColumnSpec
}

func flattenColumnSpec(columnSpecs map[string][]*columns.ColumnSpec) map[string]*columns.ColumnSpec {
	output := map[string]*columns.ColumnSpec{}
	for _, cols := range columnSpecs {
		for _, col := range cols {
			output[col.ColumnName] = col
		}
	}
	return output
}

func (meta *metadata) getColumnInfoWithIR(resolved *resolvedTrainClauseWithIR, fields []string) (map[string]*columns.ColumnSpec, error) {
	columns := map[string]*columns.ColumnSpec{}
	refColumns := flattenColumnSpec(resolved.ColumnSpecs)

	sparseColumns, _ := meta.getSparseColumnInfo()
	// TODO(joyyoj): check error if odps can support `show tables`.
	if len(sparseColumns) == 0 { // no feature mapping table.
		for _, cols := range resolved.ColumnSpecs {
			for _, col := range cols {
				if col.IsSparse {
					sparseColumns[col.ColumnName] = col
				}
			}
		}
	}
	for k, v := range sparseColumns {
		columns[k] = v
	}

	denseKeys := make([]string, 0)
	for _, key := range fields {
		_, present := columns[key]
		if !present {
			denseKeys = append(denseKeys, key)
		}
	}
	if len(denseKeys) > 0 {
		denseColumns, err := meta.getDenseColumnInfo(denseKeys, refColumns)
		if err != nil {
			log.Fatalf("Failed to get dense column %v", err)
			return columns, err
		}
		for k, v := range denseColumns {
			columns[k] = v
		}
	}
	return columns, nil
}


func getFieldsWithIR(meta *metadata, selectFields []string, label string) ([]string, error) {
	//selectFields := pr.standardSelect.fields.Strings()
	if len(selectFields) == 1 && selectFields[0] == "*" {
		selectFields = make([]string, 0)
		columnTypes, err := meta.descTable()
		if err != nil {
			return selectFields, err
		}
		for _, columnType := range columnTypes {
			if columnType.Name() != label {
				selectFields = append(selectFields, columnType.Name())
			}
		}
		return selectFields, nil
	}
	fields := make([]string, 0)
	for _, field := range selectFields {
		if field != label {
			fields = append(fields, field)
		}
	}
	return fields, nil
}

func (meta *metadata) descTable() ([]*sql.ColumnType, error) {
	// TODO(joyyoj) use `desc table`, but maxcompute not support currently.
	query := fmt.Sprintf("SELECT * FROM %s LIMIT 1", meta.table)
	sqlDB, _ := sql.Open("maxcompute", meta.odpsConfig.FormatDSN())
	rows, err := sqlDB.Query(query)

	if err != nil {
		return make([]*sql.ColumnType, 0), err
	}
	defer sqlDB.Close()
	return rows.ColumnTypes()
}

func (meta *metadata) getDenseColumnInfo(keys []string, refColumns map[string]*columns.ColumnSpec) (map[string]*columns.ColumnSpec, error) {
	output := map[string]*columns.ColumnSpec{}
	fields := strings.Join(keys, ",")
	query := fmt.Sprintf("SELECT %s FROM %s LIMIT 1", fields, meta.table)
	sqlDB, _ := sql.Open("maxcompute", meta.odpsConfig.FormatDSN())
	rows, err := sqlDB.Query(query)
	if err != nil {
		return output, err
	}
	defer sqlDB.Close()
	columnTypes, _ := rows.ColumnTypes()
	columnNamess, _ := rows.Columns()
	count := len(columnNamess)
	for rows.Next() {
		values := make([]interface{}, count)
		for i, ct := range columnTypes {
			v, e := sqlflow.CreateByType(ct.ScanType())
			if e != nil {
				return output, e
			}
			values[i] = v
		}
		if err := rows.Scan(values...); err != nil {
			return output, err
		}
		for idx, ct := range columnTypes {
			denseValue := values[idx].(*string)
			fields := strings.Split(*denseValue, ",")
			shape := make([]int, 1)
			shape[0] = len(fields)
			if userSpec, ok := refColumns[ct.Name()]; ok {
				output[ct.Name()] = &columns.ColumnSpec{
					ColumnName: ct.Name(),
					IsSparse:   false,
					Shape:      shape,
					DType:      userSpec.DType,
					Delimiter:  userSpec.Delimiter,
					Vocabulary: nil,
					FeatureMap: *meta.featureMap}
			} else {
				output[ct.Name()] = &columns.ColumnSpec{
					ColumnName: ct.Name(),
					IsSparse:   false,
					Shape:      shape,
					DType:      "float",
					Delimiter:  ",",
					Vocabulary: nil,
					FeatureMap: *meta.featureMap}
			}
		}
	}
	return output, nil
}

func (meta *metadata) getSparseColumnInfo() (map[string]*columns.ColumnSpec, error) {
	output := map[string]*columns.ColumnSpec{}

	sqlDB, _ := sql.Open("maxcompute", meta.odpsConfig.FormatDSN())
	filter := "feature_type != '' "
	if meta.featureMap.Partition != "" {
		filter += "and " + meta.featureMap.Partition
	}
	query := fmt.Sprintf("SELECT feature_type, max(cast(id as bigint)) as feature_num, group "+
		"FROM %s WHERE %s GROUP BY group, feature_type", meta.featureMap.Table, filter)

	rows, err := sqlDB.Query(query)
	if err != nil {
		return output, err
	}
	defer sqlDB.Close()
	columnTypes, _ := rows.ColumnTypes()
	columnNames, _ := rows.Columns()
	count := len(columnNames)
	for rows.Next() {
		values := make([]interface{}, count)
		for i, ct := range columnTypes {
			v, e := sqlflow.CreateByType(ct.ScanType())
			if e != nil {
				return output, e
			}
			values[i] = v
		}

		if err := rows.Scan(values...); err != nil {
			return output, err
		}
		name := values[0].(*string)
		ishape, _ := strconv.Atoi(*values[1].(*string))
		ishape++

		group := values[2].(*string)
		column, present := output[*name]
		if !present {
			shape := make([]int, 0, 1000)
			column := &columns.ColumnSpec{
				ColumnName: *name,
				IsSparse:   true,
				Shape:      shape,
				DType:      "int64",
				Delimiter:  "",
				Vocabulary: nil,
				FeatureMap: *meta.featureMap}
			column.DType = "int64"
			output[*name] = column
		}
		column, _ = output[*name]
		if *group == "\\N" {
			column.Shape = append(column.Shape, ishape)
		} else {
			igroup, _ := strconv.Atoi(*group)
			if len(column.Shape) < igroup+1 {
				column.Shape = column.Shape[0 : igroup+1]
			}
			column.Shape[igroup] = ishape
		}
	}
	return output, nil
}
