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
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sqlflow.org/gomaxcompute"
	"strconv"
	"strings"
	"text/template"
)

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

	// Feature map
	FeatureMapTable     string
	FeatureMapPartition string

	// ODPS
	OdpsConf   *gomaxcompute.Config
	EngineCode string
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

func newALPSTrainFiller(pr *extendedSelect, db *DB) (*alpsFiller, error) {
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
		modelDir = fmt.Sprintf("arks://sqlflow/%s.tar.gz", pr.trainClause.save)
	}
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
		EngineCode:          engineCode}, nil
}

func newALPSPredictFiller(pr *extendedSelect) (*alpsFiller, error) {
	return nil, fmt.Errorf("alps predict not supported")
}

func genALPSFiller(w io.Writer, pr *extendedSelect, db *DB) (*alpsFiller, error) {
	if pr.train {
		return newALPSTrainFiller(pr, db)
	}
	return newALPSPredictFiller(pr)
}

func submitALPS(w *PipeWriter, pr *extendedSelect, db *DB, cwd string) error {
	var program bytes.Buffer

	filler, err := genALPSFiller(&program, pr, db)
	if err != nil {
		return err
	}

	if err = alpsTemplate.Execute(&program, filler); err != nil {
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
	if pr.train {
		// TODO(uuleon): save model to DB
	}
	return nil
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

const alpsTemplateText = `
# coding: utf-8
# Copyright (c) Antfin, Inc. All rights reserved.

from __future__ import absolute_import
from __future__ import division
from __future__ import print_function

import os

import tensorflow as tf

from alps.conf.closure import Closure
from alps.framework.train.training import build_run_config
from alps.framework.exporter import ExportStrategy
from alps.framework.exporter.arks_exporter import ArksExporter
from alps.client.base import run_experiment, submit_experiment
from alps.framework.engine import LocalEngine, YarnEngine, ResourceConf
from alps.framework.column.column import DenseColumn, SparseColumn, GroupedSparseColumn
from alps.framework.exporter.compare_fn import best_auc_fn
from alps.io import DatasetX
from alps.io.base import OdpsConf, FeatureMap
from alps.framework.experiment import EstimatorBuilder, Experiment, TrainConf, EvalConf, RuntimeConf
from alps.io.reader.odps_reader import OdpsReader

os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'    # for debug usage.
#tf.logging.set_verbosity(tf.logging.INFO)

class SQLFlowEstimatorBuilder(EstimatorBuilder):
    def _build(self, experiment, run_config):
{{if ne .FeatureMapTable ""}}
        feature_columns = []
        {{.FeatureColumnCode}}
{{end}}
{{if ne .ImportCode ""}}
        {{.ImportCode}}
{{end}}
        return {{.ModelCreatorCode}}

if __name__ == "__main__":
    odpsConf=OdpsConf(
        accessid="{{.OdpsConf.AccessID}}",
        accesskey="{{.OdpsConf.AccessKey}}",
        endpoint="{{.OdpsConf.Endpoint}}"
    )

    trainDs = DatasetX(
        num_epochs={{.TrainClause.Epoch}},
        batch_size={{.TrainClause.BatchSize}},
        shuffle="{{.TrainClause.EnableShuffle}}" == "true",
        shuffle_buffer_size={{.TrainClause.ShuffleBufferSize}},
{{if .TrainClause.EnableCache}}
        cache_file={{.TrainClause.CachePath}},
{{end}}
        reader=OdpsReader(
            odps=odpsConf,
            project="{{.OdpsConf.Project}}",
            table="{{.TrainInputTable}}",
            # FIXME(typhoonzero): add field_names back if needed.
            # field_names={{.Fields}},
            features={{.X}},
            labels={{.Y}},
{{if ne .FeatureMapTable ""}}
            feature_map=FeatureMap(table="{{.FeatureMapTable}}",
{{if ne .FeatureMapPartition ""}}
                partition="{{.FeatureMapPartition}}"
{{end}}
            ),
            flatten_group=True
{{end}}
        ),
        drop_remainder="{{.TrainClause.DropRemainder}}" == "true"
    )

    evalDs = DatasetX(
        num_epochs=1,
        batch_size={{.TrainClause.BatchSize}},
        reader=OdpsReader(
        odps=odpsConf,
            project="{{.OdpsConf.Project}}",
            table="{{.EvalInputTable}}",
            # FIXME(typhoonzero): add field_names back if needed.
            # field_names={{.Fields}},
            features={{.X}},
            labels={{.Y}},
            flatten_group=True
        )
    )

    export_path = "{{.ModelDir}}"
{{if ne .ScratchDir ""}}
    runtime_conf = RuntimeConf(model_dir="{{.ScratchDir}}")
{{else}}
    runtime_conf = None
{{end}}
    experiment = Experiment(
        user="shangchun.sun",  # TODO(joyyoj) pai will check user name be a valid user, removed later.
        engine={{.EngineCode}},
        train=TrainConf(input=trainDs,
{{if (ne .TrainClause.MaxSteps -1)}}
                        max_steps={{.TrainClause.MaxSteps}},
{{end}}
        ),
        eval=EvalConf(input=evalDs,
                      # FIXME(typhoonzero): Support configure metrics
                      metrics_set=['accuracy'],
{{if (ne .TrainClause.EvalSteps -1)}}
                      steps={{.TrainClause.EvalSteps}},
{{end}}
                      start_delay_secs={{.TrainClause.EvalStartDelay}},
                      throttle_secs={{.TrainClause.EvalThrottle}},
        ),
        # FIXME(typhoonzero): Use ExportStrategy.BEST when possible.
        exporter=ArksExporter(deploy_path=export_path, strategy=ExportStrategy.LATEST, compare_fn=Closure(best_auc_fn)),
        runtime = runtime_conf,
        model_builder=SQLFlowEstimatorBuilder())

    if isinstance(experiment.engine, LocalEngine):
        run_experiment(experiment)
    else:
        submit_experiment(experiment, exit_on_submit=True)
`

var alpsTemplate = template.Must(template.New("alps").Parse(alpsTemplateText))

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
	selectFields := pr.standardSelect.fields
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
