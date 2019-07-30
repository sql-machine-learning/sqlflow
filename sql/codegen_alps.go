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
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	pb "github.com/sql-machine-learning/sqlflow/server/proto"
	"sqlflow.org/gomaxcompute"
)

var alpsTrainTemplate = template.Must(template.New("alps_train").Parse(alpsTrainTemplateText))
var alpsPredTemplate = template.Must(template.New("alps_predict").Parse(alpsPredTemplateText))

type alpsFiller struct {
	// Training or Predicting
	IsTraining bool

	// Input & Output
	TrainInputTable    string
	EvalInputTable     string
	PredictInputTable  string
	ModelDir           string
	ScratchDir         string
	PredictOutputTable string

	// Schema & Decode info
	Fields string
	X      string
	Y      string

	// Train
	ImportCode        string
	ModelCreatorCode  string
	FeatureColumnCode string
	TrainClause       *resolvedTrainClause
	ExitOnSubmit      bool

	// Predict
	PredictUDF string

	// Feature map
	FeatureMapTable     string
	FeatureMapPartition string

	// ODPS
	OdpsConf   *gomaxcompute.Config
	EngineCode string

	// Credential
	UserID string
	OSSID  string
	OSSKey string
}

type alpsFeatureColumn interface {
	featureColumn
	GenerateAlpsCode(metadata *metadata) ([]string, error)
}

func engineCreatorCode(resolved *resolvedTrainClause) (string, error) {
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

func modelCreatorCode(resolved *resolvedTrainClause, args []string) (string, string, error) {
	cl := make([]string, 0)
	for _, a := range resolved.ModelConstructorParams {
		code, err := a.GenerateCode()
		if err != nil {
			return "", "", err
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

	return importCode,
		fmt.Sprintf("%s(%s)", modelName, strings.Join(cl, ",")), nil
}

func newALPSTrainFiller(pr *extendedSelect, db *DB, session *pb.Session) (*alpsFiller, error) {
	resolved, err := resolveTrainClause(&pr.trainClause)
	if err != nil {
		return nil, err
	}

	var odpsConfig = &gomaxcompute.Config{}
	var columnInfo map[string]*columnSpec

	// TODO(joyyoj) read feature mapping table's name from table attributes.
	// TODO(joyyoj) pr may contains partition.
	fmap := featureMap{pr.tables[0] + "_feature_map", ""}
	var meta metadata
	fields := make([]string, 0)
	if db != nil {
		odpsConfig, err = gomaxcompute.ParseDSN(db.dataSourceName)
		if err != nil {
			return nil, err
		}
		meta = metadata{odpsConfig, pr.tables[0], &fmap, nil}
		fields, err = getFields(&meta, pr)
		if err != nil {
			return nil, err
		}
		columnInfo, err = meta.getColumnInfo(resolved, fields)
		meta.columnInfo = &columnInfo
	} else {
		meta = metadata{odpsConfig, pr.tables[0], nil, nil}
		columnInfo = map[string]*columnSpec{}
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
	y := &columnSpec{
		ColumnName: pr.label,
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
		codes, err := generateAlpsFeatureColumnCode(fcs, &meta)
		if err != nil {
			return nil, err
		}
		for _, code := range codes {
			pycode := fmt.Sprintf("feature_columns.append(%s)", code)
			featureColumnCode = append(featureColumnCode, pycode)
		}
	}
	fcCode := strings.Join(featureColumnCode, "\n        ")
	importCode, modelCode, err := modelCreatorCode(resolved, args)
	if err != nil {
		return nil, err
	}
	tableName := pr.tables[0]
	var engineCode string
	engineCode, err = engineCreatorCode(resolved)
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
		modelDir = fmt.Sprintf("arks://%s/%s.tar.gz", filepath.Join("sqlflow", userID), pr.trainClause.save)
	}
	log.Printf("Will save the models on: %s\n", modelDir)
	return &alpsFiller{
		IsTraining:          true,
		TrainInputTable:     tableName,
		EvalInputTable:      tableName, //FIXME(uuleon): Train and Eval should use different dataset.
		ScratchDir:          scratchDir,
		ModelDir:            modelDir,
		Fields:              fmt.Sprintf("[%s]", strings.Join(fields, ",")),
		X:                   fmt.Sprintf("[%s]", strings.Join(csCode, ",")),
		Y:                   y.ToString(),
		OdpsConf:            odpsConfig,
		ImportCode:          importCode,
		ModelCreatorCode:    modelCode,
		FeatureColumnCode:   fcCode,
		TrainClause:         resolved,
		FeatureMapTable:     fmap.Table,
		FeatureMapPartition: fmap.Partition,
		EngineCode:          engineCode,
		ExitOnSubmit:        exitOnSubmit}, nil
}

func newALPSPredictFiller(pr *extendedSelect, session *pb.Session) (*alpsFiller, error) {
	var ossID, ossKey *expr
	var ok bool
	if ossID, ok = pr.predAttrs["OSS_ID"]; !ok {
		return nil, fmt.Errorf("the ALPS Predict job should specify OSS_ID")
	}
	if ossKey, ok = pr.predAttrs["OSS_KEY"]; !ok {
		return nil, fmt.Errorf("the ALPS Predict job should specify OSS_KEY")
	}
	modelDir := fmt.Sprintf("oss://arks-model/%s/%s.tar.gz", session.UserId, pr.predictClause.model)

	return &alpsFiller{
		IsTraining:         true,
		PredictInputTable:  pr.tables[0],
		PredictOutputTable: pr.predictClause.into,
		PredictUDF:         strings.Join(pr.fields.Strings(), " "),
		ModelDir:           modelDir,
		UserID:             session.UserId,
		OSSID:              ossID.String(),
		OSSKey:             ossKey.String(),
	}, nil
}

func alpsTrain(w *PipeWriter, pr *extendedSelect, db *DB, cwd string, session *pb.Session) error {
	var program bytes.Buffer
	filler, err := newALPSTrainFiller(pr, db, session)
	if err != nil {
		return err
	}

	if err = alpsTrainTemplate.Execute(&program, filler); err != nil {
		return fmt.Errorf("submitALPS: failed executing template: %v", err)
	}
	code := program.String()
	cw := &logChanWriter{wr: w}
	cmd := tensorflowCmd(cwd, "maxcompute")
	filename := "experiment.py"
	absfile := filepath.Join(cwd, filename)
	f, err := os.Create(absfile)
	if err != nil {
		return fmt.Errorf("Create python code failed %v", err)
	}
	f.WriteString(program.String())
	f.Close()
	initRc := filepath.Join(cwd, "init.rc")
	initf, err := os.Create(initRc)
	if err != nil {
		return fmt.Errorf("Create init file failed %v", err)
	}
	// TODO(joyyoj) Release a stable-alps to pypi.antfin-inc.com, then remove it.
	initf.WriteString(`
#!/bin/bash
pip install http://091349.oss-cn-hangzhou-zmf.aliyuncs.com/alps/sqlflow/alps-2.0.3rc3-py2.py3-none-any.whl -i https://pypi.antfin-inc.com/simple
`)
	initf.Close()

	cmd.Args = append(cmd.Args, filename)
	cmd.Stdout = cw
	cmd.Stderr = cw
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("code %v failed %v", code, e)
	}
	// TODO(uuleon): save model to DB
	return nil
}

func alpsPred(w *PipeWriter, pr *extendedSelect, db *DB, cwd string, session *pb.Session) error {
	var program bytes.Buffer
	filler, err := newALPSPredictFiller(pr, session)
	if err != nil {
		return err
	}
	if err = alpsPredTemplate.Execute(&program, filler); err != nil {
		return fmt.Errorf("submitALPS: failed executing template: %v", err)
	}

	fname := "alps_pre.odps"
	filepath := filepath.Join(cwd, fname)
	f, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("Create ODPS script failed %v", err)
	}
	f.WriteString(program.String())
	f.Close()
	cw := &logChanWriter{wr: w}
	_, ok := db.Driver().(*gomaxcompute.Driver)
	if !ok {
		return fmt.Errorf("Alps Predict Job only supports Maxcompute database driver")
	}
	cfg, err := gomaxcompute.ParseDSN(db.dataSourceName)
	if err != nil {
		return fmt.Errorf("Parse Maxcompute DSN failed: %v", err)
	}
	// FIXME(Yancey1989): using https proto.
	fixedEndpoint := strings.Replace(cfg.Endpoint, "https://", "http://", 0)
	// TODO(Yancey1989): submit the Maxcompute UDF script using gomaxcompute driver.
	cmd := exec.Command("odpscmd",
		"-u", cfg.AccessID,
		"-p", cfg.AccessKey,
		fmt.Sprintf("--endpoint=%s", fixedEndpoint),
		fmt.Sprintf("--project=%s", cfg.Project),
		"-s", filepath)
	cmd.Dir = cwd
	cmd.Stdout = cw
	cmd.Stderr = cw
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("submit ODPS script %s failed %v", program.String(), e)
	}
	return nil
}

func submitALPS(w *PipeWriter, pr *extendedSelect, db *DB, cwd string, session *pb.Session) error {
	if pr.train {
		return alpsTrain(w, pr, db, cwd, session)
	}
	return alpsPred(w, pr, db, cwd, session)
}

func (nc *numericColumn) GenerateAlpsCode(metadata *metadata) ([]string, error) {
	output := make([]string, 0)
	output = append(output,
		fmt.Sprintf("tf.feature_column.numeric_column(\"%s\", shape=%s)", nc.Key,
			strings.Join(strings.Split(fmt.Sprint(nc.Shape), " "), ",")))
	return output, nil
}

func (bc *bucketColumn) GenerateAlpsCode(metadata *metadata) ([]string, error) {
	sourceCode, _ := bc.SourceColumn.GenerateCode()
	output := make([]string, 0)
	output = append(output, fmt.Sprintf(
		"tf.feature_column.bucketized_column(%s, boundaries=%s)",
		sourceCode,
		strings.Join(strings.Split(fmt.Sprint(bc.Boundaries), " "), ",")))
	return output, nil
}

func (cc *crossColumn) GenerateAlpsCode(metadata *metadata) ([]string, error) {
	var keysGenerated = make([]string, len(cc.Keys))
	var output []string
	for idx, key := range cc.Keys {
		if c, ok := key.(featureColumn); ok {
			code, err := c.GenerateCode()
			if err != nil {
				return output, err
			}
			keysGenerated[idx] = code
			continue
		}
		if str, ok := key.(string); ok {
			keysGenerated[idx] = fmt.Sprintf("\"%s\"", str)
		} else {
			return output, fmt.Errorf("cross generate code error, key: %s", key)
		}
	}
	output = append(output, fmt.Sprintf(
		"tf.feature_column.crossed_column([%s], hash_bucket_size=%d)",
		strings.Join(keysGenerated, ","), cc.HashBucketSize))
	return output, nil
}

func (cc *categoryIDColumn) GenerateAlpsCode(metadata *metadata) ([]string, error) {
	output := make([]string, 0)
	columnInfo, present := (*metadata.columnInfo)[cc.Key]
	var err error
	if !present {
		err = fmt.Errorf("Failed to get column info of %s", cc.Key)
	} else if len(columnInfo.Shape) == 0 {
		err = fmt.Errorf("Shape is empty %s", cc.Key)
	} else if len(columnInfo.Shape) == 1 {
		output = append(output, fmt.Sprintf("tf.feature_column.categorical_column_with_identity(key=\"%s\", num_buckets=%d)",
			cc.Key, cc.BucketSize))
	} else {
		for i := 0; i < len(columnInfo.Shape); i++ {
			output = append(output, fmt.Sprintf("tf.feature_column.categorical_column_with_identity(key=\"%s_%d\", num_buckets=%d)",
				cc.Key, i, cc.BucketSize))
		}
	}
	return output, err
}

func (cc *sequenceCategoryIDColumn) GenerateAlpsCode(metadata *metadata) ([]string, error) {
	output := make([]string, 0)
	columnInfo, present := (*metadata.columnInfo)[cc.Key]
	var err error
	if !present {
		err = fmt.Errorf("Failed to get column info of %s", cc.Key)
	} else if len(columnInfo.Shape) == 0 {
		err = fmt.Errorf("Shape is empty %s", cc.Key)
	} else if len(columnInfo.Shape) == 1 {
		output = append(output, fmt.Sprintf("tf.feature_column.sequence_categorical_column_with_identity(key=\"%s\", num_buckets=%d)",
			cc.Key, cc.BucketSize))
	} else {
		for i := 0; i < len(columnInfo.Shape); i++ {
			output = append(output, fmt.Sprintf("tf.feature_column.sequence_categorical_column_with_identity(key=\"%s_%d\", num_buckets=%d)",
				cc.Key, i, cc.BucketSize))
		}
	}
	return output, err
}

func (ec *embeddingColumn) GenerateAlpsCode(metadata *metadata) ([]string, error) {
	var output []string
	catColumn, ok := ec.CategoryColumn.(alpsFeatureColumn)
	if !ok {
		return output, fmt.Errorf("embedding generate code error, input is not featureColumn: %s", ec.CategoryColumn)
	}
	sourceCode, err := catColumn.GenerateAlpsCode(metadata)
	if err != nil {
		return output, err
	}
	output = make([]string, 0)
	for _, elem := range sourceCode {
		output = append(output, fmt.Sprintf("tf.feature_column.embedding_column(%s, dimension=%d, combiner=\"%s\")",
			elem, ec.Dimension, ec.Combiner))
	}
	return output, nil
}

func generateAlpsFeatureColumnCode(fcs []featureColumn, metadata *metadata) ([]string, error) {
	var codes = make([]string, 0, 1000)
	for _, fc := range fcs {
		code, err := fc.(alpsFeatureColumn).GenerateAlpsCode(metadata)
		if err != nil {
			return codes, nil
		}
		codes = append(codes, code...)
	}
	return codes, nil
}

type metadata struct {
	odpsConfig *gomaxcompute.Config
	table      string
	featureMap *featureMap
	columnInfo *map[string]*columnSpec
}

func flattenColumnSpec(columns map[string][]*columnSpec) map[string]*columnSpec {
	output := map[string]*columnSpec{}
	for _, cols := range columns {
		for _, col := range cols {
			output[col.ColumnName] = col
		}
	}
	return output
}

func (meta *metadata) getColumnInfo(resolved *resolvedTrainClause, fields []string) (map[string]*columnSpec, error) {
	columns := map[string]*columnSpec{}
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

// get all referenced field names.
func getAllKeys(fcs []featureColumn) []string {
	output := make([]string, 0)
	for _, fc := range fcs {
		key := fc.(alpsFeatureColumn).GetKey()
		output = append(output, key)
	}
	return output
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

func getFields(meta *metadata, pr *extendedSelect) ([]string, error) {
	selectFields := pr.standardSelect.fields.Strings()
	if len(selectFields) == 1 && selectFields[0] == "*" {
		selectFields = make([]string, 0)
		columnTypes, err := meta.descTable()
		if err != nil {
			return selectFields, err
		}
		for _, columnType := range columnTypes {
			if columnType.Name() != pr.label {
				selectFields = append(selectFields, columnType.Name())
			}
		}
		return selectFields, nil
	}
	fields := make([]string, 0)
	for _, field := range selectFields {
		if field != pr.label {
			fields = append(fields, field)
		}
	}
	return fields, nil
}

func (meta *metadata) getDenseColumnInfo(keys []string, refColumns map[string]*columnSpec) (map[string]*columnSpec, error) {
	output := map[string]*columnSpec{}
	fields := strings.Join(keys, ",")
	query := fmt.Sprintf("SELECT %s FROM %s LIMIT 1", fields, meta.table)
	sqlDB, _ := sql.Open("maxcompute", meta.odpsConfig.FormatDSN())
	rows, err := sqlDB.Query(query)
	if err != nil {
		return output, err
	}
	defer sqlDB.Close()
	columnTypes, _ := rows.ColumnTypes()
	columns, _ := rows.Columns()
	count := len(columns)
	for rows.Next() {
		values := make([]interface{}, count)
		for i, ct := range columnTypes {
			v, e := createByType(ct.ScanType())
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
				output[ct.Name()] = &columnSpec{ct.Name(), false, false, shape, userSpec.DType, userSpec.Delimiter, *meta.featureMap}
			} else {
				output[ct.Name()] = &columnSpec{ct.Name(), false, false, shape, "float", ",", *meta.featureMap}
			}
		}
	}
	return output, nil
}

func (meta *metadata) getSparseColumnInfo() (map[string]*columnSpec, error) {
	output := map[string]*columnSpec{}

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
	columns, _ := rows.Columns()
	count := len(columns)
	for rows.Next() {
		values := make([]interface{}, count)
		for i, ct := range columnTypes {
			v, e := createByType(ct.ScanType())
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
			column := &columnSpec{*name, false, true, shape, "int64", "", *meta.featureMap}
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
