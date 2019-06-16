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
	"fmt"
	"io"
	"io/ioutil"
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

	// Estimator
	EstimatorCreatorCode string
	TrainSpec            collection
	EvalSpec             collection

	// Config
	OdpsConf    collection
	DatasetConf collection
}

type collection map[string]string

func (c collection) Get(key string, fallback string) string {
	if v, ok := c[key]; ok {
		return v
	}
	return fallback
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

func generateFeatureColumnCode(fcs []featureColumn) (string, error) {
	var codes = make([]string, 0, len(fcs))
	for _, fc := range fcs {
		code, err := fc.GenerateCode()
		if err != nil {
			return "", nil
		}
		codes = append(codes, code)
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

func generateFeatureSpecCode(fs *featureSpec) string {
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

func newALPSTrainFiller(pr *extendedSelect) (*alpsFiller, error) {
	//TODO(uuleon): the scratchDir will be deleted after model uploading
	scratchDir, err := ioutil.TempDir("/tmp", "alps_scratch_dir_")
	if err != nil {
		return nil, err
	}
	modelDir := fmt.Sprintf("%s/model/", scratchDir)

	fcMap := map[string][]featureColumn{}
	fsMap := map[string]*featureSpec{}

	for target, columns := range pr.columns {
		fcs, fss, err := resolveTrainColumns(&columns)
		if err != nil {
			return nil, err
		}
		fcMap[target] = fcs
		for k, v := range fss {
			fsMap[k] = v
		}
	}

	fssCode := make([]string, 0, len(fsMap))
	for _, fs := range fsMap {
		fssCode = append(fssCode, generateFeatureSpecCode(fs))
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
	for target, fcs := range fcMap {
		code, err := generateFeatureColumnCode(fcs)
		if err != nil {
			return nil, err
		}
		args = append(args, fmt.Sprintf("%s=%s", target, code))
	}
	estimatorCode, err := generateEstimatorCreator(pr.estimator, estimatorAttrs, args)
	if err != nil {
		return nil, err
	}

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
		ScratchDir:           scratchDir,
		ModelDir:             modelDir,
		Fields:               fmt.Sprintf("[%s]", strings.Join(fields, ",")),
		X:                    fmt.Sprintf("[%s]", strings.Join(fssCode, ",")),
		Y:                    generateFeatureSpecCode(y),
		OdpsConf:             odpsMap,
		TrainSpec:            trainMap,
		EvalSpec:             evalMap,
		DatasetConf:          datasetMap,
		EstimatorCreatorCode: estimatorCode}, nil
}

func newALPSPredictFiller(pr *extendedSelect) (*alpsFiller, error) {
	return nil, fmt.Errorf("alps predict not supported")
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
	cmd := tensorflowCmd(cwd, "maxcompute")
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

from alps.conf.closure import Closure
from alps.framework.train.training import build_run_config
from alps.framework.exporter import ExportStrategy
from alps.framework.exporter.arks_exporter import ArksExporter
from alps.client.base import run_experiment
from alps.framework.engine import LocalEngine
from alps.framework.column.column import DenseColumn
from alps.framework.exporter.compare_fn import best_auc_fn
from alps.io import DatasetX
from alps.io.base import OdpsConf
from alps.framework.experiment import EstimatorBuilder, Experiment, TrainConf, EvalConf
from alps.io.reader.odps_reader import OdpsReader

os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'    # for debug usage.
tf.logging.set_verbosity(tf.logging.INFO)


class SQLFlowEstimatorBuilder(EstimatorBuilder):
    def _build(self, experiment, run_config):
		return {{.EstimatorCreatorCode}}


if __name__ == "__main__":
	
	trainDs = DatasetX(
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
		)
	)

	evalDs = DatasetX(
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
		)
	)

	export_path = "{{.ModelDir}}"

	experiment = Experiment(
		user="sqlflow",
		engine=LocalEngine(),
		train=TrainConf(input=trainDs,
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
		eval=EvalConf(input=evalDs, 
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
		exporter=ArksExporter(deploy_path=export_path, strategy=ExportStrategy.BEST, compare_fn=Closure(best_auc_fn)),
		model_dir="{{.ScratchDir}}",
		model_builder=SQLFlowEstimatorBuilder())


	run_experiment(experiment)

`

var alpsTemplate = template.Must(template.New("alps").Parse(alpsTemplateText))
