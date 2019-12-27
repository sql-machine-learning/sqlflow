# Copyright 2019 The SQLFlow Authors. All rights reserved.
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
# Disable Tensorflow INFO and WARNING logs
os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'

import sys, json
import tensorflow as tf
import functools
import copy
try:
    import sqlflow_models
except:
    pass

from sqlflow_submitter.db import connect_with_data_source, db_generator, buffered_db_writer, parseMaxComputeDSN
from .input_fn import get_dtype, parse_sparse_feature, pai_maxcompute_input_fn
from .fast_predict import FastPredict

# TODO(shendiaomo): Remove after we fully upgrade to TF2.0
TF_VERSION_2 = True
TF_VERSION_PARTS = tf.__version__.split(".")
if int(TF_VERSION_PARTS[0]) == 1:
    TF_VERSION_2 = False

# Disable Tensorflow INFO and WARNING logs
if TF_VERSION_2:
    import logging
    tf.get_logger().setLevel(logging.ERROR)
else:
    tf.logging.set_verbosity(tf.logging.ERROR)
    from .pai_distributed import define_tf_flags, make_distributed_info_without_evaluator, dump_into_tf_config

def keras_predict(estimator, model_params, save, result_table,
                  feature_column_names, feature_metas, label_meta,
                  datasource, select,
                  hdfs_namenode_addr, hive_location, hdfs_user, hdfs_pass):
    classifier = estimator(**model_params)
    classifier_pkg = sys.modules[estimator.__module__]

    conn = connect_with_data_source(datasource)
    def eval_input_fn(batch_size, cache=False):
        feature_types = []
        for name in feature_column_names:
            # NOTE: vector columns like 23,21,3,2,0,0 should use shape None
            if feature_metas[name]["is_sparse"]:
                feature_types.append((tf.int64, tf.int32, tf.int64))
            else:
                feature_types.append(get_dtype(feature_metas[name]["dtype"]))

        gen = db_generator(conn.driver, conn, select,
            feature_column_names, label_meta["feature_name"], feature_metas)
        dataset = tf.data.Dataset.from_generator(gen, (tuple(feature_types), eval("tf.%s" % label_meta["dtype"])))
        ds_mapper = functools.partial(parse_sparse_feature, feature_column_names=feature_column_names, feature_metas=feature_metas)
        dataset = dataset.map(ds_mapper).batch(batch_size)
        if cache:
            dataset = dataset.cache()
        return dataset

    # NOTE: always use batch_size=1 when predicting to get the pairs of features and predict results
    #       to insert into result table.
    pred_dataset = eval_input_fn(1)
    one_batch = next(pred_dataset)
    # NOTE: must run predict one batch to initialize parameters
    # see: https://www.tensorflow.org/alpha/guide/keras/saving_and_serializing#saving_subclassed_models
    classifier.predict_on_batch(one_batch[0])
    classifier.load_weights(save)
    pred_dataset = eval_input_fn(1, cache=True).make_one_shot_iterator()
    buff_rows = []
    column_names = feature_column_names[:]
    column_names.append(label_meta["feature_name"])
    with buffered_db_writer(conn.driver, conn, result_table, column_names, 100, hdfs_namenode_addr, hive_location, hdfs_user, hdfs_pass) as w:
        for features in pred_dataset:
            result = classifier.predict_on_batch(features[0])
            result = classifier_pkg.prepare_prediction_column(result[0])
            row = []
            for idx, name in enumerate(feature_column_names):
                val = features[0][name].numpy()[0][0]
                row.append(str(val))
            row.append(str(result))
            w.write(row)
    del pred_dataset

def estimator_predict(estimator, model_params, save, result_table,
                  feature_column_names, feature_metas, label_meta,
                  datasource, select,
                  hdfs_namenode_addr, hive_location, hdfs_user, hdfs_pass,
                  is_pai, pai_table):
    classifier = estimator(**model_params)
    conn = connect_with_data_source(datasource)

    def fast_input_fn(generator):
        feature_types = []
        for name in feature_column_names:
            if feature_metas[name]["is_sparse"]:
                feature_types.append((tf.int64, tf.int32, tf.int64))
            else:
                feature_types.append(get_dtype(feature_metas[name]["dtype"]))

        def _inner_input_fn():
            if is_pai:
                dataset = pai_maxcompute_input_fn(pai_table, datasource,
                            feature_column_names, feature_metas, label_meta)
            else:
                dataset = tf.data.Dataset.from_generator(generator, (tuple(feature_types), eval("tf.%s" % label_meta["dtype"])))
                ds_mapper = functools.partial(parse_sparse_feature, feature_column_names=feature_column_names, feature_metas=feature_metas)
                dataset = dataset.map(ds_mapper)
            dataset = dataset.batch(1).cache()
            iterator = dataset.make_one_shot_iterator()
            features = iterator.get_next()
            return features

        return _inner_input_fn

    column_names = feature_column_names[:]
    column_names.append(label_meta["feature_name"])
    fast_predictor = FastPredict(classifier, fast_input_fn)

    with buffered_db_writer(conn.driver, conn, result_table, column_names, 100, hdfs_namenode_addr, hive_location, hdfs_user, hdfs_pass) as w:
        for features in db_generator(conn.driver, conn, select, feature_column_names, label_meta["feature_name"], feature_metas)():
            result = fast_predictor.predict(features)
            row = []
            for idx, _ in enumerate(feature_column_names):
                val = features[0][idx][0]
                row.append(str(val))
            if "class_ids" in list(result)[0]:
                row.append(str(list(result)[0]["class_ids"][0]))
            else:
                # regression predictions
                row.append(str(list(result)[0]["predictions"][0]))
            w.write(row)

def pred(is_keras_model,
         datasource,
         estimator,
         select,
         result_table,
         feature_columns,
         feature_column_names,
         feature_metas={},
         label_meta={},
         model_params={},
         save="",
         batch_size=1,
         hdfs_namenode_addr="",
         hive_location="",
         hdfs_user="",
         hdfs_pass="",
         is_pai=False,
         pai_table=""):
    if not is_pai:
        conn = connect_with_data_source(datasource)
    model_params.update(feature_columns)

    if is_keras_model:
        if not issubclass(estimator, tf.keras.Model):
            # functional model need field_metas parameter
            model_params["field_metas"] = feature_metas
        keras_predict(estimator, model_params, save, result_table,
            feature_column_names, feature_metas, label_meta,
            datasource, select,
            hdfs_namenode_addr, hive_location, hdfs_user, hdfs_pass)
    else:
        if is_pai:
            FLAGS = define_tf_flags()
            model_params["model_dir"] = FLAGS.checkpointDir
        else:
            model_params['model_dir'] = save
        estimator_predict(estimator, model_params, save, result_table,
                feature_column_names, feature_metas, label_meta,
                datasource, select,
                hdfs_namenode_addr, hive_location, hdfs_user, hdfs_pass,
                is_pai, pai_table)

    print("Done predicting. Predict table : %s" % result_table)
