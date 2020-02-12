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

import copy
import functools
import json
import os
import sys

import numpy as np
import tensorflow as tf
from sqlflow_submitter.db import (buffered_db_writer, connect_with_data_source,
                                  db_generator, parseMaxComputeDSN)

from .input_fn import (get_dtype, pai_maxcompute_db_generator,
                       parse_sparse_feature, parse_sparse_feature_predict)

# Disable Tensorflow INFO and WARNING logs
os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'

try:
    import sqlflow_models
except:
    pass

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
                  feature_column_names, feature_metas, result_col_name,
                  datasource, select, hdfs_namenode_addr, hive_location,
                  hdfs_user, hdfs_pass):
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

        gen = db_generator(conn.driver, conn, select, feature_column_names,
                           None, feature_metas)
        dataset = tf.data.Dataset.from_generator(gen, (tuple(feature_types), ))
        ds_mapper = functools.partial(
            parse_sparse_feature_predict,
            feature_column_names=feature_column_names,
            feature_metas=feature_metas)
        dataset = dataset.map(ds_mapper).batch(batch_size)
        if cache:
            dataset = dataset.cache()
        return dataset

    # NOTE: always use batch_size=1 when predicting to get the pairs of features and predict results
    #       to insert into result table.
    pred_dataset = eval_input_fn(1)
    one_batch = next(iter(pred_dataset))
    # NOTE: must run predict one batch to initialize parameters
    # see: https://www.tensorflow.org/alpha/guide/keras/saving_and_serializing#saving_subclassed_models
    classifier.predict_on_batch(one_batch)
    classifier.load_weights(save)
    pred_dataset = eval_input_fn(1, cache=True).make_one_shot_iterator()
    buff_rows = []
    column_names = feature_column_names[:]
    column_names.append(result_col_name)
    with buffered_db_writer(conn.driver, conn, result_table, column_names, 100,
                            hdfs_namenode_addr, hive_location, hdfs_user,
                            hdfs_pass) as w:
        for features in pred_dataset:
            result = classifier.predict_on_batch(features)
            result = classifier_pkg.prepare_prediction_column(result[0])
            row = []
            for idx, name in enumerate(feature_column_names):
                val = features[name].numpy()[0][0]
                row.append(str(val))
            if isinstance(result, np.ndarray):
                if len(result) > 1:
                    # NOTE(typhoonzero): if the output dimension > 1, format output tensor
                    # using a comma separated string. Only available for keras models.
                    row.append(",".join([str(i) for i in result]))
                else:
                    row.append(str(result[0]))
            else:
                row.append(str(result))
            w.write(row)
    del pred_dataset


def estimator_predict(estimator, model_params, save, result_table,
                      feature_column_names, feature_metas, result_col_name,
                      datasource, select, hdfs_namenode_addr, hive_location,
                      hdfs_user, hdfs_pass, is_pai, pai_table):
    if not is_pai:
        conn = connect_with_data_source(datasource)

    column_names = feature_column_names[:]
    column_names.append(result_col_name)

    if is_pai:
        driver = "pai_maxcompute"
        conn = None
        pai_table_parts = pai_table.split(".")
        formated_pai_table = "odps://%s/tables/%s" % (pai_table_parts[0],
                                                      pai_table_parts[1])
        predict_generator = pai_maxcompute_db_generator(
            formated_pai_table, feature_column_names, None, feature_metas)()
    else:
        driver = conn.driver
        predict_generator = db_generator(conn.driver, conn, select,
                                         feature_column_names, None,
                                         feature_metas)()
    # load from the exported model
    if save.startswith("oss://"):
        with open("exported_path", "r") as fn:
            export_path = fn.read()
        parts = save.split("?")
        export_path_oss = parts[0] + export_path
        if TF_VERSION_2:
            imported = tf.saved_model.load(export_path_oss)
        else:
            imported = tf.saved_model.load_v2(export_path_oss)
    else:
        with open("exported_path", "r") as fn:
            export_path = fn.read()
        if TF_VERSION_2:
            imported = tf.saved_model.load(export_path)
        else:
            imported = tf.saved_model.load_v2(export_path)

    def predict(x):
        example = tf.train.Example()
        for i in range(len(feature_column_names)):
            feature_name = feature_column_names[i]
            dtype_str = feature_metas[feature_name]["dtype"]
            # if dtype_str == "float32" or dtype_str == "float64":
            if feature_metas[feature_name]["delimiter"] != "":
                print(x[0][i])
                if dtype_str == "float32" or dtype_str == "float64":
                    example.features.feature[
                        feature_name].float_list.value.extend(
                            list(x[0][i].flatten()))
                elif dtype_str == "int32" or dtype_str == "int64":
                    example.features.feature[
                        feature_name].int64_list.value.extend(
                            list(x[0][i].flatten()))
            else:
                example.features.feature[feature_name].float_list.value.extend(
                    x[0][i])
            # elif dtype_str == "int32" or dtype_str == "int64":
            #     example.features.feature[feature_name].int64_list.value.extend(
            #         x[0][i])
        return imported.signatures["predict"](
            examples=tf.constant([example.SerializeToString()]))

    with buffered_db_writer(driver, conn, result_table, column_names, 100,
                            hdfs_namenode_addr, hive_location, hdfs_user,
                            hdfs_pass) as w:
        for features in predict_generator:
            result = predict(features)
            row = []
            for idx, _ in enumerate(feature_column_names):
                val = features[0][idx][0]
                row.append(str(val))
            if "class_ids" in result:
                row.append(str(result["class_ids"].numpy()[0][0]))
            else:
                # regression predictions
                row.append(str(result["predictions"].numpy()[0][0]))
            w.write(row)


def pred(datasource,
         estimator,
         select,
         result_table,
         feature_columns,
         feature_column_names,
         result_col_name,
         feature_metas={},
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

    is_estimator = issubclass(
        estimator,
        (tf.estimator.Estimator, tf.estimator.BoostedTreesClassifier,
         tf.estimator.BoostedTreesRegressor))
    if not is_estimator:
        if not issubclass(estimator, tf.keras.Model):
            # functional model need field_metas parameter
            model_params["field_metas"] = feature_metas
        keras_predict(estimator, model_params, save, result_table,
                      feature_column_names, feature_metas, result_col_name,
                      datasource, select, hdfs_namenode_addr, hive_location,
                      hdfs_user, hdfs_pass)
    else:
        if is_pai:
            FLAGS = define_tf_flags()
            model_params["model_dir"] = FLAGS.checkpointDir
        else:
            model_params['model_dir'] = save
        estimator_predict(estimator, model_params, save, result_table,
                          feature_column_names, feature_metas, result_col_name,
                          datasource, select, hdfs_namenode_addr,
                          hive_location, hdfs_user, hdfs_pass, is_pai,
                          pai_table)

    print("Done predicting. Predict table : %s" % result_table)
