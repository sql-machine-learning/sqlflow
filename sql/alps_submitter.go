package sql

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"text/template"
)

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
	TableName            string
	Fields               string
	X                    string
	Y                    string
	OdpsConf             map[string]string
	TrainSpec            map[string]string
	DatasetConf          map[string]string
	FeatureColumnCode    string
	EstimatorCreatorCode string
	ScratchDir           string
}

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
	if str, ok := resolved.(string); !ok {
		return "", fmt.Errorf("expression expected to be string, actual: %s", resolved)
	}
	return str, nil
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

func generateEstimatorCreator(estimator string, attrs []*attribute) (string, error) {
	cl := make([]string, len(attrs))
	for idx, a := range attrs {
		code, err := a.GenerateCode()
		if err != nil {
			return "", err
		}
		cl[idx] = code
	}
	return fmt.Sprintf("tf.estimator.%s(%s)", estimator, strings.Join(cl, ",")), nil
}

func newALPSTrainFiller(pr *extendedSelect) (*alpsFiller, error) {
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

	datasetAttrs := filter(attrs, "dataset")
	datasetMap := make(map[string]string, len(datasetAttrs))
	for _, a := range datasetAttrs {
		datasetMap[a.Name] = a.Value.(string)
	}

	estimatorAttrs := filter(attrs, "estimator")

	estimatorCode, err := generateEstimatorCreator(pr.estimator, estimatorAttrs)
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

	dir, err := ioutil.TempDir("/tmp", "alps_scratch_dir")
	if err != nil {
		return nil, err
	}

	return &alpsFiller{
		TableName:            tableName,
		Fields:               fmt.Sprintf("[%s]", strings.Join(fields, ",")),
		X:                    fmt.Sprintf("[%s]", strings.Join(fssCode, ",")),
		Y:                    y.ToString(),
		OdpsConf:             odpsMap,
		TrainSpec:            trainMap,
		DatasetConf:          datasetMap,
		FeatureColumnCode:    fcCode,
		EstimatorCreatorCode: estimatorCode,
		ScratchDir:           dir}, nil
}

func genALPSTrain(w io.Writer, pr *extendedSelect) error {
	r, e := newALPSTrainFiller(pr)
	if e != nil {
		return e
	}
	if e = alpsTemplate.Execute(w, r); e != nil {
		return fmt.Errorf("genALPSTrain: failed executing template: %v", e)
	}
	return nil
}

func trainALPS(wr *PipeWriter, pr *extendedSelect, cwd string) error {
	var program bytes.Buffer
	if e := genALPSTrain(&program, pr); e != nil {
		return e
	}

	code := program.String()

	cw := &logChanWriter{wr: wr}
	cmd := tensorflowCmd(cwd)
	cmd.Stdin = &program
	cmd.Stdout = cw
	cmd.Stderr = cw
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("code %v training failed %v", code, e)
	}
	return nil
}

func submitALPS(w *PipeWriter, pr *extendedSelect, db *DB, cwd string) error {
	if pr.train {
		return trainALPS(w, pr, cwd)
	}
	return fmt.Errorf("inference not supported yet in ALPS")
}

const alpsTemplateText = `
# coding: utf-8
# Copyright (c) Antfin, Inc. All rights reserved.

from __future__ import absolute_import
from __future__ import division
from __future__ import print_function

import os

import tensorflow as tf

from alps.client.base import submit_experiment, run_experiment
from alps.conf.engine import LocalEngine
from alps.estimator.column import SparseColumn, DenseColumn, RawColumn
from alps.estimator.experiment import Experiment, TrainConf, EvalConf, EstimatorBuilder
from alps.estimator.feature_column import embedding_column
from alps.io.alps_dataset import AlpsDataset
from alps.io.odps_io import OdpsConf
from alps.io.reader.pyodps_reader import OdpsReader

os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'    # for debug usage.
tf.logging.set_verbosity(tf.logging.INFO)


class SQLFlowEstimatorBuilder(EstimatorBuilder):
    def build(self, experiment):
        feature_columns = []
		{{if .FeatureColumnCode}}
        feature_columns.extend({{.FeatureColumnCode}})
		{{end}}

        return {{.EstimatorCreatorCode}}


if __name__ == "__main__":	

    ds = AlpsDataset(
        num_epochs={{.DatasetConf.epoch}},
		batch_size={{.DatasetConf.batch_size}},
        reader=OdpsReader(
            odps=OdpsConf(
				accessid={{.OdpsConf.accessid}},
				accesskey={{.OdpsConf.accesskey}},
				endpoint={{.OdpsConf.endpoint}}
			),
            project={{.OdpsConf.project}},
            table="{{.TableName}}",
            field_names={{.Fields}},
            features={{.X}},
            labels={{.Y}}
        )
    )

    experiment = Experiment(
        user="sqlflow",
		engine=LocalEngine(),
		{{if .TrainSpec.max_steps}}
        train=TrainConf(input=ds, max_steps={{.TrainSpec.max_steps}}),
		{{else}}
		train=TrainConf(input=ds),
		{{end}}
        scratch_dir="{{.ScratchDir}}",
        estimator=SQLFlowEstimatorBuilder())

    run_experiment(experiment)

`

var alpsTemplate = template.Must(template.New("alps").Parse(alpsTemplateText))
