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

import os
import shutil

import tensorflow as tf

from alps.framework.column.column import (DenseColumn, GroupedSparseColumn,
                                          SparseColumn)
from alps.framework.engine import LocalEngine
from alps.framework.experiment import EstimatorBuilder
from alps.io.base import OdpsConf
from sqlflow_submitter.alps.train import train


class SQLFlowEstimatorBuilder(EstimatorBuilder):
    def _build(self, experiment, run_config):
        feature_columns = []
        feature_columns.append(
            tf.feature_column.embedding_column(
                tf.feature_column.categorical_column_with_identity(
                    key="deep_id_0", num_buckets=15033),
                dimension=512,
                combiner="mean",
                initializer=None))
        feature_columns.append(
            tf.feature_column.embedding_column(
                tf.feature_column.categorical_column_with_identity(
                    key="user_space_stat_0", num_buckets=310),
                dimension=64,
                combiner="mean",
                initializer=None))
        feature_columns.append(
            tf.feature_column.embedding_column(
                tf.feature_column.categorical_column_with_identity(
                    key="user_behavior_stat_0", num_buckets=511),
                dimension=64,
                combiner="mean",
                initializer=None))
        feature_columns.append(
            tf.feature_column.embedding_column(
                tf.feature_column.categorical_column_with_identity(
                    key="space_stat_0", num_buckets=418),
                dimension=64,
                combiner="mean",
                initializer=None))
        return tf.estimator.DNNClassifier(n_classes=2,
                                          hidden_units=[10, 20],
                                          config=run_config,
                                          feature_columns=feature_columns)


if __name__ == "__main__":
    odps_project = os.getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
    odps_conf = OdpsConf(
        accessid=os.getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_AK"),
        accesskey=os.getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_SK"),
        # endpoint should looks like: "https://service.cn.maxcompute.aliyun.com/api"
        endpoint=os.getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_ENDPOINT"),
        project=odps_project)
    features = [
        SparseColumn(name="deep_id", shape=[15033], dtype="int"),
        SparseColumn(name="user_space_stat", shape=[310], dtype="int"),
        SparseColumn(name="user_behavior_stat", shape=[511], dtype="int"),
        SparseColumn(name="space_stat", shape=[418], dtype="int")
    ]
    labels = DenseColumn(name="l", shape=[1], dtype="int", separator=",")
    try:
        os.mkdir("scratch")
    except FileExistsError:
        pass
    train(SQLFlowEstimatorBuilder(),
          odps_conf=odps_conf,
          project=odps_project,
          train_table="gomaxcompute_driver_w7u.sparse_column_test",
          eval_table="gomaxcompute_driver_w7u.sparse_column_test",
          features=features,
          labels=labels,
          feature_map_table="",
          feature_map_partition="",
          epochs=1,
          batch_size=2,
          shuffle=False,
          shuffle_bufsize=128,
          cache_file="",
          max_steps=1000,
          eval_steps=100,
          eval_batch_size=1,
          eval_start_delay=120,
          eval_throttle=600,
          drop_remainder=True,
          export_path="./scratch/model",
          scratch_dir="./scratch",
          user_id="",
          engine_config={"name": "LocalEngine"},
          exit_on_submit=False)
    shutil.rmtree("scratch")
