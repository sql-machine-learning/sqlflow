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
import inspect
import json
import os
import sys

import numpy as np
import sqlflow_submitter
import tensorflow as tf
from sqlflow_submitter import db
from sqlflow_submitter.pai import model
from sqlflow_submitter.tensorflow.get_tf_model_type import is_tf_estimator
from tensorflow.estimator import (BoostedTreesClassifier,
                                  BoostedTreesRegressor, DNNClassifier,
                                  DNNLinearCombinedClassifier,
                                  DNNLinearCombinedRegressor, DNNRegressor,
                                  LinearClassifier, LinearRegressor)

from .get_tf_version import tf_is_version2
from .input_fn import get_dtype, parse_sparse_feature_predict, tf_generator
from .keras_with_feature_column_input import WrappedKerasModel

try:
    import sqlflow_models
except:
    pass

# Disable Tensorflow INFO and WARNING logs
os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'

# Disable Tensorflow INFO and WARNING logs
if tf_is_version2():
    import logging
    tf.get_logger().setLevel(logging.ERROR)
else:
    tf.logging.set_verbosity(tf.logging.ERROR)
    from .pai_distributed import (define_tf_flags,
                                  make_distributed_info_without_evaluator,
                                  dump_into_tf_config)


def keras_predict(estimator, model_params, save, result_table, is_pai,
                  pai_table, feature_column_names, feature_metas,
                  result_col_name, datasource, select, hdfs_namenode_addr,
                  hive_location, hdfs_user, hdfs_pass):
    signature = inspect.signature(estimator)
    has_feature_columns_arg = False
    for p in signature.parameters:
        if signature.parameters[p].name == "feature_columns":
            has_feature_columns_arg = True
            break
    if not has_feature_columns_arg:
        feature_columns = model_params["feature_columns"]
        del model_params["feature_columns"]
        classifier = WrappedKerasModel(estimator, model_params,
                                       feature_columns)
    else:
        classifier = estimator(**model_params)
    classifier_pkg = sys.modules[estimator.__module__]
    conn = None
    if is_pai:
        driver = "pai_maxcompute"
    else:
        conn = db.connect_with_data_source(datasource)
        driver = conn.driver

    def eval_input_fn(batch_size, cache=False):
        feature_types = []
        for name in feature_column_names:
            # NOTE: vector columns like 23,21,3,2,0,0 should use shape None
            if feature_metas[name]["is_sparse"]:
                feature_types.append((tf.int64, tf.int32, tf.int64))
            else:
                feature_types.append(get_dtype(feature_metas[name]["dtype"]))

        if is_pai:
            pai_table_parts = pai_table.split(".")
            formatted_pai_table = "odps://%s/tables/%s" % (pai_table_parts[0],
                                                           pai_table_parts[1])
            gen = db.pai_maxcompute_db_generator(formatted_pai_table,
                                                 feature_column_names, None,
                                                 feature_metas)
            selected_cols = feature_column_names
        else:
            gen = db.db_generator(driver, conn, select, feature_column_names,
                                  None, feature_metas)
            selected_cols = db.selected_cols(driver, conn, select)
        tf_gen = tf_generator(gen, selected_cols, feature_column_names,
                              feature_metas)
        dataset = tf.data.Dataset.from_generator(tf_gen,
                                                 (tuple(feature_types), ))
        ds_mapper = functools.partial(
            parse_sparse_feature_predict,
            feature_column_names=feature_column_names,
            feature_metas=feature_metas)
        dataset = dataset.map(ds_mapper).batch(batch_size)
        if cache:
            dataset = dataset.cache()
        return dataset

    # NOTE: always use batch_size=1 when predicting to get the pairs of
    #       features and predict results to insert into result table.
    pred_dataset = eval_input_fn(1)
    one_batch = next(iter(pred_dataset))
    # NOTE: must run predict one batch to initialize parameters
    # see: https://www.tensorflow.org/alpha/guide/keras/saving_and_serializing#saving_subclassed_models
    classifier.predict_on_batch(one_batch)
    classifier.load_weights(save)
    pred_dataset = eval_input_fn(1, cache=True).make_one_shot_iterator()
    column_names = feature_column_names[:]
    column_names.append(result_col_name)

    with db.buffered_db_writer(driver, conn, result_table, column_names, 100,
                               hdfs_namenode_addr, hive_location, hdfs_user,
                               hdfs_pass) as w:
        for features in pred_dataset:
            result = classifier.predict_on_batch(features)
            # FIXME(typhoonzero): determine the predict result is classification by
            # adding the prediction result together to see if it is close to 1.0.
            if len(result[0]) == 1:  # regression result
                result = result[0][0]
            else:
                sum = 0
                for i in result[0]:
                    sum += i
                if np.isclose(sum, 1.0):  # classification result
                    result = result[0].argmax(axis=-1)
                else:
                    result = result[0]  # multiple regression result
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


def write_cols_from_selected(result_col_name, selected_cols):
    write_cols = selected_cols[:]
    if result_col_name in selected_cols:
        target_col_index = selected_cols.index(result_col_name)
        del write_cols[target_col_index]
    else:
        target_col_index = -1
    # always keep the target column to be the last column
    # on writing prediction result
    write_cols.append(result_col_name)
    return write_cols, target_col_index


def estimator_predict(estimator, model_params, save, result_table,
                      feature_column_names, feature_column_names_map,
                      feature_columns, feature_metas, result_col_name,
                      datasource, select, hdfs_namenode_addr, hive_location,
                      hdfs_user, hdfs_pass, is_pai, pai_table):
    if not is_pai:
        conn = db.connect_with_data_source(datasource)
    column_names = feature_column_names[:]
    column_names.append(result_col_name)

    if is_pai:
        driver = "pai_maxcompute"
        conn = None
        pai_table_parts = pai_table.split(".")
        formatted_pai_table = "odps://%s/tables/%s" % (pai_table_parts[0],
                                                       pai_table_parts[1])
        selected_cols = db.pai_selected_cols(formatted_pai_table)
        predict_generator = db.pai_maxcompute_db_generator(
            formatted_pai_table, feature_column_names, None, feature_metas)()

    else:
        driver = conn.driver

        # bypass all selected cols to the prediction result table
        selected_cols = db.selected_cols(conn.driver, conn, select)
        predict_generator = db.db_generator(conn.driver, conn, select,
                                            feature_column_names, None,
                                            feature_metas)()

    write_cols, target_col_index = write_cols_from_selected(
        result_col_name, selected_cols)
    # load from the exported model
    with open("exported_path", "r") as fn:
        export_path = fn.read()
    if tf_is_version2():
        imported = tf.saved_model.load(export_path)
    else:
        imported = tf.saved_model.load_v2(export_path)

    def add_to_example(example, x, i):
        feature_name = feature_column_names[i]
        dtype_str = feature_metas[feature_name]["dtype"]
        if feature_metas[feature_name]["delimiter"] != "":
            if feature_metas[feature_name]["is_sparse"]:
                # NOTE(typhoonzero): sparse feature will get (indices,values,shape) here, use indices only
                values = x[0][i][0].flatten()
            else:
                values = x[0][i].flatten()
            if dtype_str == "float32" or dtype_str == "float64":
                example.features.feature[feature_name].float_list.value.extend(
                    list(values))
            elif dtype_str == "int32" or dtype_str == "int64":
                example.features.feature[feature_name].int64_list.value.extend(
                    list(values))
        else:
            if "feature_columns" in feature_columns:
                idx = feature_column_names.index(feature_name)
                fc = feature_columns["feature_columns"][idx]
            else:
                # DNNLinearCombinedXXX have dnn_feature_columns and linear_feature_columns param.
                idx = -1
                try:
                    idx = feature_column_names_map[
                        "dnn_feature_columns"].index(feature_name)
                    fc = feature_columns["dnn_feature_columns"][idx]
                except:
                    try:
                        idx = feature_column_names_map[
                            "linear_feature_columns"].index(feature_name)
                        fc = feature_columns["linear_feature_columns"][idx]
                    except:
                        pass
                if idx == -1:
                    raise ValueError(
                        "can not found feature %s in all feature columns")
            if dtype_str == "float32" or dtype_str == "float64":
                # need to pass a tuple(float, )
                example.features.feature[feature_name].float_list.value.extend(
                    (float(x[0][i][0]), ))
            elif dtype_str == "int32" or dtype_str == "int64":
                numeric_type = type(tf.feature_column.numeric_column("tmp"))
                if type(fc) == numeric_type:
                    example.features.feature[
                        feature_name].float_list.value.extend(
                            (float(x[0][i][0]), ))
                else:
                    example.features.feature[
                        feature_name].int64_list.value.extend(
                            (int(x[0][i][0]), ))
            elif dtype_str == "string":
                example.features.feature[feature_name].bytes_list.value.extend(
                    x[0][i])

    def predict(x):
        example = tf.train.Example()
        for i in range(len(feature_column_names)):
            add_to_example(example, x, i)
        return imported.signatures["predict"](
            examples=tf.constant([example.SerializeToString()]))

    with db.buffered_db_writer(driver, conn, result_table, write_cols, 100,
                               hdfs_namenode_addr, hive_location, hdfs_user,
                               hdfs_pass) as w:
        for row, _ in predict_generator:
            features = db.read_features_from_row(row, selected_cols,
                                                 feature_column_names,
                                                 feature_metas)
            result = predict((features, ))
            if target_col_index != -1:
                del row[target_col_index]
            if "class_ids" in result:
                row.append(str(result["class_ids"].numpy()[0][0]))
            else:
                # regression predictions
                row.append(str(result["predictions"].numpy()[0][0]))
            w.write(row)


def pred(datasource,
         estimator_string,
         select,
         result_table,
         feature_columns,
         feature_column_names,
         feature_column_names_map,
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
    # import custom model package
    sqlflow_submitter.import_model_def(estimator_string, globals())
    estimator = eval(estimator_string)

    model_params.update(feature_columns)

    is_estimator = is_tf_estimator(estimator)

    if not is_estimator:
        if not issubclass(estimator, tf.keras.Model):
            # functional model need field_metas parameter
            model_params["field_metas"] = feature_metas
        print("Start predicting using keras model...")
        keras_predict(estimator, model_params, save, result_table, is_pai,
                      pai_table, feature_column_names, feature_metas,
                      result_col_name, datasource, select, hdfs_namenode_addr,
                      hive_location, hdfs_user, hdfs_pass)
    else:
        model_params['model_dir'] = save
        print("Start predicting using estimator model...")
        estimator_predict(estimator, model_params, save, result_table,
                          feature_column_names, feature_column_names_map,
                          feature_columns, feature_metas, result_col_name,
                          datasource, select, hdfs_namenode_addr,
                          hive_location, hdfs_user, hdfs_pass, is_pai,
                          pai_table)

    print("Done predicting. Predict table : %s" % result_table)
