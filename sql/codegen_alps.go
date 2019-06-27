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

	"sqlflow.org/gomaxcompute"
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
	ModelCreatorCode string
	TrainClause      *resolvedTrainClause

	// Feature map
	FeatureMapTable     string
	FeatureMapPartition string

	// ODPS
	OdpsConf *gomaxcompute.Config
}

func modelCreatorCode(resolved *resolvedTrainClause, args []string) (string, error) {
	cl := make([]string, 0)
	for _, a := range resolved.ModelConstructorParams {
		code, err := a.GenerateCode()
		if err != nil {
			return "", err
		}
		cl = append(cl, code)
	}
	if args != nil {
		for _, arg := range args {
			cl = append(cl, arg)
		}
	}
	modelName := resolved.ModelName
	if resolved.IsPreMadeModel {
		modelName = fmt.Sprintf("tf.estimator.%s", resolved.ModelName)
	}
	return fmt.Sprintf("%s(%s)", modelName, strings.Join(cl, ",")), nil
}

func newALPSTrainFiller(pr *extendedSelect, db *DB) (*alpsFiller, error) {
	resolved, err := resolveTrainClause(&pr.trainClause)
	if err != nil {
		return nil, err
	}

	featureMapTable := ""
	featureMapPartition := ""

	csCode := make([]string, 0)
	for _, css := range resolved.ColumnSpecs {
		for _, cs := range css {
			csCode = append(csCode, cs.ToString())
			if cs.FeatureMap.Table != "" {
				featureMapTable = cs.FeatureMap.Table
			}
			if cs.FeatureMap.Partition != "" {
				featureMapPartition = cs.FeatureMap.Partition
			}
		}
	}

	var odpsConfig = &gomaxcompute.Config{}
	if db != nil {
		odpsConfig, err = gomaxcompute.ParseDSN(db.dataSourceName)
		if err != nil {
			return nil, err
		}
	}

	args := make([]string, 0)
	args = append(args, "config=run_config")
	for target, fcs := range resolved.FeatureColumns {
		code, err := generateFeatureColumnCode(fcs)
		if err != nil {
			return nil, err
		}
		args = append(args, fmt.Sprintf("%s=%s", target, code))
	}
	modelCode, err := modelCreatorCode(resolved, args)
	if err != nil {
		return nil, err
	}

	tableName := pr.tables[0]

	fields := make([]string, len(pr.fields))
	for idx, f := range pr.fields {
		fields[idx] = fmt.Sprintf("\"%s\"", f)
	}

	y := &columnSpec{
		ColumnName: pr.label,
		IsSparse:   false,
		Shape:      []int{1},
		DType:      "int",
		Delimiter:  ","}

	//TODO(uuleon): the scratchDir will be deleted after model uploading
	scratchDir, err := ioutil.TempDir("/tmp", "alps_scratch_dir_")
	if err != nil {
		return nil, err
	}
	modelDir := fmt.Sprintf("%s/model/", scratchDir)

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
		ModelCreatorCode:    modelCode,
		TrainClause:         resolved,
		FeatureMapTable:     featureMapTable,
		FeatureMapPartition: featureMapPartition}, nil
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
from alps.framework.column.column import DenseColumn, SparseColumn
from alps.framework.exporter.compare_fn import best_auc_fn
from alps.io import DatasetX
from alps.io.base import OdpsConf, FeatureMap
from alps.framework.experiment import EstimatorBuilder, Experiment, TrainConf, EvalConf
from alps.io.reader.odps_reader import OdpsReader

os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'    # for debug usage.
tf.logging.set_verbosity(tf.logging.INFO)

class SQLFlowEstimatorBuilder(EstimatorBuilder):
    def _build(self, experiment, run_config):
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
            )
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
            labels={{.Y}}
        )
    )

    export_path = "{{.ModelDir}}"

    experiment = Experiment(
        user="sqlflow",
        engine=LocalEngine(),
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
        model_dir="{{.ScratchDir}}",
        model_builder=SQLFlowEstimatorBuilder())


    run_experiment(experiment)

`

var alpsTemplate = template.Must(template.New("alps").Parse(alpsTemplateText))
