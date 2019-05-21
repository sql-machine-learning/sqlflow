package sql

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/template"
)

/*
	FIXME(uuleon)

	Attention:

	In general, the `model_dir` should be a temporary directory for each `train` job who saves the output model to DB, then
	the `predict` job restores model from DB.

	But, the `maxCompute` type of DB which `ALPS` used is not supported yet, so a fixed `model_dir` is used here.

	The `train` job clear the `model_dir` at first before training, then the `predict` job restores the output model of last `train` job.

*/
const model_dir = "/tmp/alps_model_dir/"

const (
	sparse    = "SPARSE"
	numeric   = "NUMERIC"
	cross     = "CROSS"
	bucket    = "BUCKET"
	square    = "SQUARE"
	dense     = "DENSE"
	estimator = "ESTIMATOR"
	comma     = "COMMA"
)

type alpsFiller struct {
	// Training or Predicting
	IsTraining bool

	// Input & Output
	TrainInputTable    string
	EvalInputTable     string
	PredictInputTable  string
	ModelDir           string
	PredictOutputTable string

	// Schema & Decode info
	Fields string
	X      string
	Y      string

	// Estimator
	EstimatorCreatorCode string
	FeatureColumnCode    string
	TrainSpec            collection
	EvalSpec             collection

	// Config
	OdpsConf    collection
	DatasetConf collection
}

type collection map[string]string

type featureSpec struct {
	FeatureName string
	IsSparse    bool
	Shape       []int
	DType       string
	Delimiter   string
}

type featureColumn interface {
	GenerateCode() (string, error)
}

type numericColumn struct {
	Key   string
	Shape int
}

type bucketColumn struct {
	SourceColumn *numericColumn
	Boundaries   []int
}

type crossColumn struct {
	Keys           []interface{}
	HashBucketSize int
}

type attribute struct {
	FullName string
	Prefix   string
	Name     string
	Value    interface{}
}

func (c collection) Get(key string, fallback string) string {
	if v, ok := c[key]; ok {
		return v
	}
	return fallback
}

func (nc *numericColumn) GenerateCode() (string, error) {
	return fmt.Sprintf("tf.feature_column.numeric_column(\"%s\", shape=(%d,))", nc.Key, nc.Shape), nil
}

func (bc *bucketColumn) GenerateCode() (string, error) {
	sourceCode, _ := bc.SourceColumn.GenerateCode()
	return fmt.Sprintf(
		"tf.feature_column.bucketized_column(%s, boundaries=%s)",
		sourceCode,
		strings.Join(strings.Split(fmt.Sprint(bc.Boundaries), " "), ",")), nil
}

func (cc *crossColumn) GenerateCode() (string, error) {
	var keysGenerated = make([]string, len(cc.Keys))
	for idx, key := range cc.Keys {
		if c, ok := key.(featureColumn); ok {
			code, err := c.GenerateCode()
			if err != nil {
				return "", err
			}
			keysGenerated[idx] = code
			continue
		}
		if str, ok := key.(string); ok {
			keysGenerated[idx] = fmt.Sprintf("\"%s\"", str)
		} else {
			return "", fmt.Errorf("cross generate code error, key: %s", key)
		}
	}
	return fmt.Sprintf(
		"tf.feature_column.crossed_column([%s], hash_bucket_size=%d)",
		strings.Join(keysGenerated, ","), cc.HashBucketSize), nil
}

func (a *attribute) GenerateCode() (string, error) {
	if val, ok := a.Value.(string); ok {
		return fmt.Sprintf("%s=\"%s\"", a.Name, val), nil
	}
	if val, ok := a.Value.([]interface{}); ok {
		intList, err := transformToIntList(val)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s=%s", a.Name,
			strings.Join(strings.Split(fmt.Sprint(intList), " "), ",")), nil
	}
	return "", fmt.Errorf("value of attribute must be string or list of int, given %s", a.Value)
}

func (fs *featureSpec) ToString() string {
	if fs.IsSparse {
		return fmt.Sprintf("SparseColumn(name=\"%s\", shape=%s, dtype=\"%s\", separator=\"%s\")",
			fs.FeatureName,
			strings.Join(strings.Split(fmt.Sprint(fs.Shape), " "), ","),
			fs.DType,
			fs.Delimiter)
	}
	return fmt.Sprintf("DenseColumn(name=\"%s\", shape=%s, dtype=\"%s\", separator=\"%s\")",
		fs.FeatureName,
		strings.Join(strings.Split(fmt.Sprint(fs.Shape), " "), ","),
		fs.DType,
		fs.Delimiter)
}

func resolveFeatureSpec(el *exprlist, isSparse bool) (*featureSpec, error) {
	if len(*el) != 4 {
		return nil, fmt.Errorf("bad FeatureSpec expression format: %s", *el)
	}
	name, err := expression2string((*el)[1])
	if err != nil {
		return nil, fmt.Errorf("bad FeatureSpec name: %s, err: %s", (*el)[1], err)
	}
	shape, err := strconv.Atoi((*el)[2].val)
	if err != nil {
		return nil, fmt.Errorf("bad FeatureSpec shape: %s, err: %s", (*el)[2].val, err)
	}
	unresolvedDelimiter, err := expression2string((*el)[3])
	if err != nil {
		return nil, fmt.Errorf("bad FeatureSpec delimiter: %s, err: %s", (*el)[1], err)
	}

	delimiter, err := resolveDelimiter(unresolvedDelimiter)
	if err != nil {
		return nil, err
	}
	// TODO(uuleon): hard coded dtype(float) should be removed
	return &featureSpec{
		FeatureName: name,
		IsSparse:    isSparse,
		Shape:       []int{shape},
		DType:       "float",
		Delimiter:   delimiter}, nil
}

func resolveExpression(e interface{}) (interface{}, error) {
	if expr, ok := e.(*expr); ok {
		if expr.val != "" {
			return expr.val, nil
		}
		return resolveExpression(&expr.sexp)
	}

	el, ok := e.(*exprlist)
	if !ok {
		return nil, fmt.Errorf("input of resolveExpression must be `expr` or `exprlist`: %s", e)
	}

	headName := (*el)[0].val
	if headName == "" {
		return resolveExpression(&(*el)[0].sexp)
	}

	headName = strings.ToUpper(headName)

	switch headName {
	case dense:
		return resolveFeatureSpec(el, false)
	case sparse:
		return resolveFeatureSpec(el, true)
	case numeric:
		if len(*el) != 3 {
			return nil, fmt.Errorf("bad NUMERIC expression format: %s", *el)
		}
		key, err := expression2string((*el)[1])
		if err != nil {
			return nil, fmt.Errorf("bad NUMERIC key: %s, err: %s", (*el)[1], err)
		}
		shape, err := strconv.Atoi((*el)[2].val)
		if err != nil {
			return nil, fmt.Errorf("bad NUMERIC shape: %s, err: %s", (*el)[2].val, err)
		}
		return &numericColumn{
			Key:   key,
			Shape: shape}, nil
	case bucket:
		if len(*el) != 3 {
			return nil, fmt.Errorf("bad BUCKET expression format: %s", *el)
		}
		sourceExprList := (*el)[1].sexp
		boundariesExprList := (*el)[2].sexp
		source, err := resolveExpression(&sourceExprList)
		if err != nil {
			return nil, err
		}
		if _, ok := source.(*numericColumn); !ok {
			return nil, fmt.Errorf("key of BUCKET must be NUMERIC, which is %s", source)
		}
		boundaries, err := resolveExpression(&boundariesExprList)
		if err != nil {
			return nil, err
		}
		b, err := transformToIntList(boundaries.([]interface{}))
		if err != nil {
			return nil, fmt.Errorf("bad BUCKET boundaries: %s", err)
		}
		return &bucketColumn{
			SourceColumn: source.(*numericColumn),
			Boundaries:   b}, nil
	case cross:
		if len(*el) != 3 {
			return nil, fmt.Errorf("bad CROSS expression format: %s", *el)
		}
		keysExpr := (*el)[1].sexp
		keys, err := resolveExpression(&keysExpr)
		if err != nil {
			return nil, err
		}
		bucketSize, err := strconv.Atoi((*el)[2].val)
		if err != nil {
			return nil, fmt.Errorf("bad CROSS bucketSize: %s, err: %s", (*el)[2].val, err)
		}
		return &crossColumn{
			Keys:           keys.([]interface{}),
			HashBucketSize: bucketSize}, nil
	case square:
		var list []interface{}
		for idx, expr := range *el {
			if idx > 0 {
				if expr.sexp == nil {
					intVal, err := strconv.Atoi(expr.val)
					if err != nil {
						list = append(list, expr.val)
					} else {
						list = append(list, intVal)
					}
				} else {
					value, err := resolveExpression(&expr.sexp)
					if err != nil {
						return nil, err
					}
					list = append(list, value)
				}
			}
		}
		return list, nil
	default:
		return nil, fmt.Errorf("not supported expr in ALPS submitter: %s", headName)
	}
}

func transformToIntList(list []interface{}) ([]int, error) {
	var b = make([]int, len(list))
	for idx, item := range list {
		if intVal, ok := item.(int); ok {
			b[idx] = intVal
		} else {
			return nil, fmt.Errorf("type is not int: %s", item)
		}
	}
	return b, nil
}

func expression2string(e interface{}) (string, error) {
	resolved, err := resolveExpression(e)
	if err != nil {
		return "", err
	}
	if str, ok := resolved.(string); ok {
		return str, nil
	}
	return "", fmt.Errorf("expression expected to be string, actual: %s", resolved)
}

func filter(attrs []*attribute, prefix string) []*attribute {
	ret := make([]*attribute, 0)
	for _, a := range attrs {
		if strings.EqualFold(a.Prefix, prefix) {
			ret = append(ret, a)
		}
	}
	return ret
}

func resolveTrainColumns(columns *exprlist) ([]interface{}, map[string]*featureSpec, error) {
	var fsMap = make(map[string]*featureSpec)
	var fcList = make([]interface{}, 0)
	for _, expr := range *columns {
		result, err := resolveExpression(expr)
		if err != nil {
			return nil, nil, err
		}
		if fs, ok := result.(*featureSpec); ok {
			fsMap[fs.FeatureName] = fs
			continue
		}
		if c, ok := result.(featureColumn); ok {
			fcList = append(fcList, c)
		} else {
			return nil, nil, fmt.Errorf("not recgonized type: %s", result)
		}
	}
	return fcList, fsMap, nil
}

func resolveTrainAttribute(attrs *attrs) ([]*attribute, error) {
	var ret []*attribute
	for k, v := range *attrs {
		subs := strings.SplitN(k, ".", 2)
		name := subs[len(subs)-1]
		prefix := ""
		if len(subs) == 2 {
			prefix = subs[0]
		}
		r, err := resolveExpression(v)
		if err != nil {
			return nil, err
		}
		ret = append(ret, &attribute{
			FullName: k,
			Prefix:   prefix,
			Name:     name,
			Value:    r})
	}
	return ret, nil
}

func resolveDelimiter(delimiter string) (string, error) {
	if strings.EqualFold(delimiter, comma) {
		return ",", nil
	}
	return "", fmt.Errorf("unsolved delimiter: %s", delimiter)
}

func generateFeatureColumnCode(fcs []interface{}) (string, error) {
	var codes = make([]string, 0, len(fcs))
	for _, fc := range fcs {
		if fc, ok := fc.(featureColumn); ok {
			code, err := fc.GenerateCode()
			if err != nil {
				return "", nil
			}
			codes = append(codes, code)
		} else {
			return "", fmt.Errorf("input is not featureColumn interface")
		}
	}
	return fmt.Sprintf("[%s]", strings.Join(codes, ",")), nil
}

func generateEstimatorCreator(estimator string, attrs []*attribute, args []string) (string, error) {
	cl := make([]string, len(attrs))
	for idx, a := range attrs {
		code, err := a.GenerateCode()
		if err != nil {
			return "", err
		}
		cl[idx] = code
	}
	if args != nil {
		for _, arg := range args {
			cl = append(cl, arg)
		}
	}
	return fmt.Sprintf("tf.estimator.%s(%s)", estimator, strings.Join(cl, ",")), nil
}

func newALPSTrainFiller(pr *extendedSelect) (*alpsFiller, error) {
	modelDir := model_dir

	fcList, fsMap, err := resolveTrainColumns(&pr.columns)
	if err != nil {
		return nil, err
	}

	fssCode := make([]string, 0, len(fsMap))
	for _, fs := range fsMap {
		fssCode = append(fssCode, fs.ToString())
	}

	fcCode, err := generateFeatureColumnCode(fcList)
	if err != nil {
		return nil, err
	}

	attrs, err := resolveTrainAttribute(&pr.attrs)
	if err != nil {
		return nil, err
	}

	//FIXME(uuleon): need removed and parse it from odps datasource
	odpsAttrs := filter(attrs, "odps")
	odpsMap := make(map[string]string, len(odpsAttrs))
	for _, a := range odpsAttrs {
		odpsMap[a.Name] = a.Value.(string)
	}

	trainSpecAttrs := filter(attrs, "train_spec")
	trainMap := make(map[string]string, len(trainSpecAttrs))
	for _, a := range trainSpecAttrs {
		trainMap[a.Name] = a.Value.(string)
	}

	evalSpecAttrs := filter(attrs, "eval_spec")
	evalMap := make(map[string]string, len(evalSpecAttrs))
	for _, a := range evalSpecAttrs {
		evalMap[a.Name] = a.Value.(string)
	}

	datasetAttrs := filter(attrs, "dataset")
	datasetMap := make(map[string]string, len(datasetAttrs))
	for _, a := range datasetAttrs {
		datasetMap[a.Name] = a.Value.(string)
	}

	estimatorAttrs := filter(attrs, "estimator")

	args := make([]string, 0)
	args = append(args, "config=run_config")
	estimatorCode, err := generateEstimatorCreator(pr.estimator, estimatorAttrs, args)
	if err != nil {
		return nil, err
	}

	estimatorCode = strings.Replace(estimatorCode, "\"FC\"", "feature_columns", 1)

	tableName := pr.tables[0]

	fields := make([]string, len(pr.fields))
	for idx, f := range pr.fields {
		fields[idx] = fmt.Sprintf("\"%s\"", f)
	}

	y := &featureSpec{
		FeatureName: pr.label,
		IsSparse:    false,
		Shape:       []int{1},
		DType:       "int",
		Delimiter:   ","}

	return &alpsFiller{
		IsTraining:           true,
		TrainInputTable:      tableName,
		EvalInputTable:       tableName, //FIXME(uuleon): Train and Eval should use different dataset.
		ModelDir:             modelDir,
		Fields:               fmt.Sprintf("[%s]", strings.Join(fields, ",")),
		X:                    fmt.Sprintf("[%s]", strings.Join(fssCode, ",")),
		Y:                    y.ToString(),
		OdpsConf:             odpsMap,
		TrainSpec:            trainMap,
		EvalSpec:             evalMap,
		DatasetConf:          datasetMap,
		FeatureColumnCode:    fcCode,
		EstimatorCreatorCode: estimatorCode}, nil
}

func newALPSPredictFiller(pr *extendedSelect) (*alpsFiller, error) {
	modelDir := model_dir

	inputTableName := pr.tables[0]
	outputTableName := pr.predictClause.into

	fields := make([]string, len(pr.fields))
	for idx, f := range pr.fields {
		fields[idx] = fmt.Sprintf("\"%s\"", f)
	}

	return &alpsFiller{
		IsTraining:         false,
		PredictInputTable:  inputTableName,
		PredictOutputTable: outputTableName,
		ModelDir:           modelDir,
		Fields:             fmt.Sprintf("[%s]", strings.Join(fields, ","))}, nil
}

func genALPSFiller(w io.Writer, pr *extendedSelect) (*alpsFiller, error) {
	if pr.train {
		return newALPSTrainFiller(pr)
	}
	return newALPSPredictFiller(pr)
}

func submitALPS(w *PipeWriter, pr *extendedSelect, db *DB, cwd string) error {
	var program bytes.Buffer

	filler, err := genALPSFiller(&program, pr)
	if err != nil {
		return err
	}

	if err = alpsTemplate.Execute(&program, filler); err != nil {
		return fmt.Errorf("submitALPS: failed executing template: %v", err)
	}

	code := program.String()

	cw := &logChanWriter{wr: w}
	cmd := tensorflowCmd(cwd, "maxCompute")
	cmd.Stdin = &program
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

const alpsTemplateText = `
# coding: utf-8
# Copyright (c) Antfin, Inc. All rights reserved.

from __future__ import absolute_import
from __future__ import division
from __future__ import print_function

import os

import tensorflow as tf

from alps.conf import Configurable
from alps.conf.closure import Closure
from alps.framework.exporter import ExportStrategy, FileLocation
from alps.framework.exporter.arks_exporter import ArksExporter
from alps.client.base import run_experiment
from alps.framework.engine import LocalEngine
from alps.framework.column.column import DenseColumn
from alps.framework.exporter.compare_fn import best_auc_fn
from alps.io.base import OdpsConf
from alps.framework.experiment import EstimatorBuilder, Experiment, TrainConf, EvalConf, InferConf, RuntimeConf
from alps.io.alps_dataset import AlpsDataset
from alps.io.reader.odps_reader import OdpsReader
from alps.io.writer.odps_writer import OdpsConfigurableWriter

os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'    # for debug usage.
tf.logging.set_verbosity(tf.logging.INFO)


class SQLFlowEstimatorBuilder(EstimatorBuilder):
	def _build(self, experiment, run_config):
		feature_columns = []
{{if .FeatureColumnCode}}
		feature_columns.extend({{.FeatureColumnCode}})
{{end}}

		return {{.EstimatorCreatorCode}}


{{if .IsTraining}}

def train(model_dir):
	train_ds = AlpsDataset(
{{if .DatasetConf.epoch}}
		num_epochs={{.DatasetConf.epoch}},
{{end}}
{{if .DatasetConf.batch_size}}
		batch_size={{.DatasetConf.batch_size}},
{{end}}
		reader=OdpsReader(
			odps=OdpsConf(
				accessid={{.OdpsConf.Get "accessid" "None"}},
				accesskey={{.OdpsConf.Get "accesskey" "None"}},
				endpoint={{.OdpsConf.Get "endpoint" "None"}}
			),
			project={{.OdpsConf.Get "project" "None"}},
			table="{{.TrainInputTable}}",
			field_names={{.Fields}},
			features={{.X}},
			labels={{.Y}}
		),
		drop_remainder=False
	)

	eval_ds = AlpsDataset(
		num_epochs=1,
		batch_size=64,
		reader=OdpsReader(
			odps=OdpsConf(
				accessid={{.OdpsConf.Get "accessid" "None"}},
				accesskey={{.OdpsConf.Get "accesskey" "None"}},
				endpoint={{.OdpsConf.Get "endpoint" "None"}}
			),
			project={{.OdpsConf.Get "project" "None"}},
			table="{{.EvalInputTable}}",
			field_names={{.Fields}},
			features={{.X}},
			labels={{.Y}}
		),
		drop_remainder=False
	)

	deploy_path = FileLocation(path=os.path.join(model_dir, 'deploy'))

	experiment = Experiment(
		user="sqlflow",
		engine=LocalEngine(),
		train=TrainConf(input=train_ds,
{{if .TrainSpec.max_steps}}
						max_steps={{.TrainSpec.max_steps}},
{{end}}
{{if .TrainSpec.save_summary_steps}}
						save_summary_steps={{.TrainSpec.save_summary_steps}},
{{end}}
{{if .TrainSpec.save_timeline_steps}}
						save_timeline_steps={{.TrainSpec.save_timeline_steps}},
{{end}}
{{if .TrainSpec.save_checkpoints_steps}}
						save_checkpoints_steps={{.TrainSpec.save_checkpoints_steps}},
{{end}}
{{if .TrainSpec.log_step_count_steps}}
						log_step_count_steps={{.TrainSpec.log_step_count_steps}}
{{end}}
		),
		eval=EvalConf(input=eval_ds, 
{{if .TrainSpec.steps}}
					  steps={{.TrainSpec.steps}}, 
{{end}}
{{if .TrainSpec.start_delay_secs}}
					  start_delay_secs={{.TrainSpec.start_delay_secs}},
{{end}}
{{if .TrainSpec.throttle_secs}}
					  throttle_secs={{.TrainSpec.throttle_secs}},
{{end}}
{{if .TrainSpec.throttle_steps}}
 					  throttle_steps={{.TrainSpec.throttle_steps}}
{{end}}
		),
		exporter=ArksExporter(deploy_path=deploy_path, strategy=ExportStrategy.BEST, compare_fn=Closure(best_auc_fn)),
		runtime=RuntimeConf(
			model_dir=model_dir,
		),
		model_builder=SQLFlowEstimatorBuilder()
	)

	return experiment

{{else}}

def inference(model_dir):
	model_dir = os.path.join(model_dir, 'deploy')
	meta = Configurable.from_file(os.path.join(model_dir, 'alps.meta'), clazz=Experiment)
	project = meta.train.input.reader.project
	odps = meta.train.input.reader.odps
	features=meta.train.input.reader.features
	labels=meta.train.input.reader.labels

	infer_ds = AlpsDataset(
        num_epochs=1,
        batch_size=64,
        reader=OdpsReader(
            odps=odps,
			project=project,
			table="{{.PredictInputTable}}",
			field_names={{.Fields}},
            features=features,
            labels=labels,
            return_other_values=True),
		drop_remainder=False
	)

	odps_writer = OdpsConfigurableWriter(
        odps=odps,
		project=project,
		table="{{.PredictOutputTable}}",
        overwrite=True,
    )

	experiment = Experiment(user="sqlflow",
                            engine=LocalEngine(),
                            infer=InferConf(input=infer_ds,
                                            model_dir=model_dir,
                                            writer=odps_writer,
                                            signature_key="predict"),
                            runtime=RuntimeConf(
                                model_dir=model_dir,
                            ),
                            model_builder=None)
	return experiment
	
{{end}}

if __name__ == "__main__":
	model_dir = "{{.ModelDir}}"
{{if .IsTraining}}
	from distutils.dir_util import mkpath, remove_tree
	mkpath(model_dir)
	remove_tree(model_dir, verbose=0)

	run_experiment(train(model_dir))
{{else}}
	run_experiment(inference(model_dir))
{{end}}

`

var alpsTemplate = template.Must(template.New("alps").Parse(alpsTemplateText))
