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

const alpsTrainTemplateText = `
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
from alps.io import DatasetX
from alps.io.base import OdpsConf, FeatureMap
from alps.framework.experiment import EstimatorBuilder, Experiment, TrainConf, EvalConf, RuntimeConf
from alps.io.reader.odps_reader import OdpsReader
from alps.util.remote_module import RemoteModule
from alps.framework.exporter.base import MetricComparator, Goal

os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'    # for debug usage.
#tf.logging.set_verbosity(tf.logging.INFO)

class SQLFlowEstimatorBuilder(EstimatorBuilder):
    def _build(self, experiment, run_config):
{{if ne .FeatureMapTable ""}}
        feature_columns = []
        {{.FeatureColumnCode}}
{{end}}
{{if ne .RemoteModuleCode ""}}
        {{.RemoteModuleCode}}
{{end}}
{{if ne .ImportCode ""}}
        {{.ImportCode}}
{{end}}
        return {{.ModelCreatorCode}}

if __name__ == "__main__":
    odpsConf=OdpsConf(
        accessid="{{.OdpsConf.AccessID}}",
        accesskey="{{.OdpsConf.AccessKey}}",
        endpoint="{{.OdpsConf.Endpoint}}",
        project="{{.OdpsConf.Project}}"
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
        batch_size={{.TrainClause.EvalBatchSize}},
        reader=OdpsReader(
        odps=odpsConf,
            project="{{.OdpsConf.Project}}",
            table="{{.EvalInputTable}}",
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
        user="{{.UserID}}",
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
        exporter=ArksExporter(deploy_path=export_path, strategy=ExportStrategy.LATEST, compare=MetricComparator("auc", Goal.MAXIMIZE)),
        arbitrary_evaluator=True,
        runtime = runtime_conf,
        model_builder=SQLFlowEstimatorBuilder())

    if isinstance(experiment.engine, LocalEngine):
        run_experiment(experiment)
    else:
        if "{{.ExitOnSubmit}}" == "false":
            submit_experiment(experiment)
        else:
            submit_experiment(experiment, exit_on_submit=True)
`

const alpsPredTemplateText = `
set odps.task.major.version=default;
set odps.isolation.session.enable=true;
set odps.service.mode=off;
set odps.instance.priority = 0;
set odps.sql.udf.timeout = 3000;

set mst.model.path={{.ModelDir}};
set mst.model.name=tf_model;
set mst.oss.id={{.OSSID}};
set mst.oss.key={{.OSSKey}};
set mst.oss.endpoint={{.OSSEndpoint}};
set mst.load.feature_map=false;

set deepbreath.sparse.group.separator=:;
set deepbreath.sparse.separator=,;
set deepbreath.enable.sigmoid=false;
set odps.sql.mapper.split.size=64;

DROP TABLE IF EXISTS {{.PredictOutputTable}};

CREATE TABLE IF NOT EXISTS {{.PredictOutputTable}} AS SELECT {{.PredictUDF}} FROM {{.PredictInputTable}};
`
