# coding: utf-8
# Copyright 2020 The SQLFlow Authors. All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

from __future__ import absolute_import, division, print_function

import os

import tensorflow as tf
from alps.client.base import run_experiment, submit_experiment
from alps.conf.closure import Closure
from alps.framework.column.column import (DenseColumn, GroupedSparseColumn,
                                          SparseColumn)
from alps.framework.engine import (KubemakerEngine, LocalEngine, ResourceConf,
                                   YarnEngine)
from alps.framework.experiment import (EstimatorBuilder, EvalConf, Experiment,
                                       RuntimeConf, TrainConf)
from alps.framework.exporter import ExportStrategy
from alps.framework.exporter.arks_exporter import ArksExporter
from alps.framework.exporter.base import Goal, MetricComparator
from alps.framework.train.training import build_run_config
from alps.io import DatasetX
from alps.io.base import FeatureMap, OdpsConf
from alps.io.reader.odps_reader import OdpsReader
from alps.util.remote_module import RemoteModule
from tensorflow.python.saved_model.signature_constants import \
    DEFAULT_SERVING_SIGNATURE_DEF_KEY

# for debug usage.
os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'

sample_engine_config = {
    "name": "YarnEngine",
    "cluster": "clustername",
    "queue": "queuename",
    "ps_mem": 100,
    "ps_num": 2,
    "worker_mem": 1024,
    "worker_num": 10
}


def train(model_builder,
          odps_conf=None,
          project="",
          train_table="",
          eval_table="",
          features=[],
          labels=[],
          feature_map_table="",
          feature_map_partition="",
          epochs=1,
          batch_size=1,
          shuffle=False,
          shuffle_bufsize=0,
          cache_file="",
          max_steps=None,
          eval_steps=None,
          eval_batch_size=1,
          eval_start_delay=120,
          eval_throttle=600,
          drop_remainder=True,
          export_path="",
          scratch_dir="",
          user_id="",
          engine_config={"name": "LocalEngine"},
          exit_on_submit=False):
    if feature_map_table != "":
        feature_map = FeatureMap(table=feature_map_table,
                                 partition=feature_map_partition)
    else:
        feature_map = None

    trainDs = DatasetX(num_epochs=epochs,
                       batch_size=batch_size,
                       shuffle=shuffle,
                       shuffle_buffer_size=shuffle_bufsize,
                       cache_file=cache_file,
                       reader=OdpsReader(odps=odps_conf,
                                         project=project,
                                         table=train_table,
                                         features=features,
                                         labels=labels,
                                         feature_map=feature_map,
                                         flatten_group=True),
                       drop_remainder=drop_remainder)

    evalDs = DatasetX(num_epochs=1,
                      batch_size=eval_batch_size,
                      reader=OdpsReader(odps=odps_conf,
                                        project=project,
                                        table=eval_table,
                                        features=features,
                                        labels=labels,
                                        flatten_group=True))

    if scratch_dir != "":
        runtime_conf = RuntimeConf(model_dir=scratch_dir)
    else:
        runtime_conf = None

    if max_steps == None:
        keep_checkpoint_max = 100
    else:
        keep_checkpoint_max = int(max_steps / 100)

    if engine_config["name"] == "LocalEngine":
        engine = LocalEngine()
    elif engine_config["name"] == "YarnEngine":
        engine = YarnEngine(cluster=engine_config["cluster"],
                            queue=engine_config["queue"],
                            ps=ResourceConf(memory=engine_config["ps_mem"],
                                            num=engine_config["ps_num"]),
                            worker=ResourceConf(
                                memory=engine_config["worker_mem"],
                                num=engine_config["worker_num"]))
    elif engine_config["name"] == "KubemakerEngine":
        engine = KubemakerEngine(
            cluster=engine_config["cluster"],
            queue=engine_config["queue"],
            ps=ResourceConf(memory=engine_config["ps_mem"],
                            num=engine_config["ps_num"]),
            worker=ResourceConf(memory=engine_config["worker_mem"],
                                num=engine_config["worker_num"]))
    else:
        print("unknown engine type: %s" % engine_config["name"])
        exit(1)

    experiment = Experiment(
        user=user_id,
        engine=engine,
        train=TrainConf(
            input=trainDs,
            max_steps=max_steps,
            keep_checkpoint_max=keep_checkpoint_max,
        ),
        eval=EvalConf(
            input=evalDs,
            # FIXME(typhoonzero): Support configure metrics
            metrics_set=['accuracy'],
            steps=eval_steps,
            start_delay_secs=eval_start_delay,
            throttle_secs=eval_throttle,
        ),
        # FIXME(typhoonzero): Use ExportStrategy.BEST when possible.
        exporter=ArksExporter(signature_def_key='predict',
                              deploy_path=export_path,
                              strategy=ExportStrategy.LATEST,
                              compare=MetricComparator("auc", Goal.MAXIMIZE)),
        arbitrary_evaluator=True,
        runtime=runtime_conf,
        model_builder=model_builder)

    if isinstance(experiment.engine, LocalEngine):
        run_experiment(experiment)
    else:
        submit_experiment(experiment, exit_on_submit=exit_on_submit)
