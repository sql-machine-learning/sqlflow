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

import functools
import os

import numpy as np
import tensorflow as tf
from runtime import db
from runtime.feature.field_desc import DataType
from runtime.tensorflow.get_tf_model_type import is_tf_estimator
from runtime.tensorflow.get_tf_version import tf_is_version2
from runtime.tensorflow.import_model import import_model
from runtime.tensorflow.input_fn import (get_dtype,
                                         parse_sparse_feature_predict,
                                         tf_generator)
from runtime.tensorflow.keras_with_feature_column_input import \
    init_model_with_feature_column
from runtime.tensorflow.load_model import (load_keras_model_weights,
                                           pop_optimizer_and_loss)

# Disable TensorFlow INFO and WARNING logs
os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'

# Disable TensorFlow INFO and WARNING logs
if tf_is_version2():
    import logging
    tf.get_logger().setLevel(logging.ERROR)
else:
    tf.logging.set_verbosity(tf.logging.ERROR)


def keras_predict(estimator, model_params, save, result_table,
                  feature_column_names, feature_metas, train_label_name,
                  result_col_name, conn, predict_generator, selected_cols):
    pop_optimizer_and_loss(model_params)
    classifier = init_model_with_feature_column(estimator, model_params)

    def eval_input_fn(batch_size, cache=False):
        feature_types = []
        for name in feature_column_names:
            # NOTE: vector columns like 23,21,3,2,0,0 should use shape None
            if feature_metas[name]["is_sparse"]:
                feature_types.append((tf.int64, tf.int32, tf.int64))
            else:
                feature_types.append(get_dtype(feature_metas[name]["dtype"]))
        tf_gen = tf_generator(predict_generator, selected_cols,
                              feature_column_names, feature_metas)
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

    if not hasattr(classifier, 'sqlflow_predict_one'):
        # NOTE: load_weights should be called by keras models only.
        # NOTE: always use batch_size=1 when predicting to get the pairs of
        #       features and predict results to insert into result table.
        pred_dataset = eval_input_fn(1)
        one_batch = next(iter(pred_dataset))
        # NOTE: must run predict one batch to initialize parameters. See:
        # https://www.tensorflow.org/alpha/guide/keras/saving_and_serializing#saving_subclassed_models  # noqa: E501
        classifier.predict_on_batch(one_batch)
        load_keras_model_weights(classifier, save)
    pred_dataset = eval_input_fn(1, cache=True).make_one_shot_iterator()

    column_names = selected_cols[:]
    try:
        train_label_index = selected_cols.index(train_label_name)
    except:  # noqa: E722
        train_label_index = -1
    if train_label_index != -1:
        del column_names[train_label_index]
    column_names.append(result_col_name)

    with db.buffered_db_writer(conn, result_table, column_names, 100) as w:
        for features in pred_dataset:
            if hasattr(classifier, 'sqlflow_predict_one'):
                result = classifier.sqlflow_predict_one(features)
            else:
                result = classifier.predict_on_batch(features)
            # FIXME(typhoonzero): determine the predict result is
            # classification by adding the prediction result together
            # to see if it is close to 1.0.
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
                    # NOTE(typhoonzero): if the output dimension > 1, format
                    # output tensor using a comma separated string. Only
                    # available for keras models.
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


def estimator_predict(result_table, feature_column_names, feature_metas,
                      train_label_name, result_col_name, conn,
                      predict_generator, selected_cols):
    write_cols = selected_cols[:]
    try:
        train_label_index = selected_cols.index(train_label_name)
    except ValueError:
        train_label_index = -1
    if train_label_index != -1:
        del write_cols[train_label_index]
    write_cols.append(result_col_name)

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
            if feature_metas[feature_name]["delimiter_kv"] != "":
                keys = x[0][i][0].flatten()
                weights = x[0][i][1].flatten()
                weight_dtype_str = feature_metas[feature_name]["dtype_weight"]
                if (dtype_str == "float32" or dtype_str == "float64"
                        or dtype_str == DataType.FLOAT32):
                    raise ValueError(
                        "not supported key-value feature with key type float")
                elif (dtype_str == "int32" or dtype_str == "int64"
                      or dtype_str == DataType.INT64):
                    example.features.feature[
                        feature_name].int64_list.value.extend(list(keys))
                elif (dtype_str == "string" or dtype_str == DataType.STRING):
                    example.features.feature[
                        feature_name].bytes_list.value.extend(list(keys))
                if (weight_dtype_str == "float32"
                        or weight_dtype_str == "float64"
                        or weight_dtype_str == DataType.FLOAT32):
                    example.features.feature["_".join(
                        [feature_name,
                         "weight"])].float_list.value.extend(list(weights))
                else:
                    raise ValueError(
                        "not supported key value column weight data type: %s" %
                        weight_dtype_str)
            else:
                # NOTE(typhoonzero): sparse feature will get
                # (indices,values,shape) here, use indices only
                values = x[0][i][0].flatten()
                if (dtype_str == "float32" or dtype_str == "float64"
                        or dtype_str == DataType.FLOAT32):
                    example.features.feature[
                        feature_name].float_list.value.extend(list(values))
                elif (dtype_str == "int32" or dtype_str == "int64"
                      or dtype_str == DataType.INT64):
                    example.features.feature[
                        feature_name].int64_list.value.extend(list(values))
        else:
            if (dtype_str == "float32" or dtype_str == "float64"
                    or dtype_str == DataType.FLOAT32):
                # need to pass a tuple(float, )
                example.features.feature[feature_name].float_list.value.extend(
                    (float(x[0][i][0]), ))
            elif (dtype_str == "int32" or dtype_str == "int64"
                  or dtype_str == DataType.INT64):
                example.features.feature[feature_name].int64_list.value.extend(
                    (int(x[0][i][0]), ))
            elif dtype_str == "string" or dtype_str == DataType.STRING:
                example.features.feature[feature_name].bytes_list.value.extend(
                    x[0][i])

    def predict(x):
        example = tf.train.Example()
        for i in range(len(feature_column_names)):
            add_to_example(example, x, i)
        return imported.signatures["predict"](
            examples=tf.constant([example.SerializeToString()]))

    with db.buffered_db_writer(conn, result_table, write_cols, 100) as w:
        for row, _ in predict_generator():
            features = db.read_features_from_row(row,
                                                 selected_cols,
                                                 feature_column_names,
                                                 feature_metas,
                                                 is_xgboost=False)
            result = predict((features, ))
            if train_label_index != -1 and len(row) > train_label_index:
                del row[train_label_index]
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
         train_label_name,
         result_col_name,
         feature_metas={},
         model_params={},
         save="",
         batch_size=1):
    estimator = import_model(estimator_string)
    model_params.update(feature_columns)
    is_estimator = is_tf_estimator(estimator)

    conn = db.connect_with_data_source(datasource)
    predict_generator = db.db_generator(conn, select)
    selected_cols = db.selected_cols(conn, select)

    if not is_estimator:
        if not issubclass(estimator, tf.keras.Model):
            # functional model need field_metas parameter
            model_params["field_metas"] = feature_metas
        print("Start predicting using keras model...")
        keras_predict(estimator, model_params, save, result_table,
                      feature_column_names, feature_metas, train_label_name,
                      result_col_name, conn, predict_generator, selected_cols)
    else:
        model_params['model_dir'] = save
        print("Start predicting using estimator model...")
        estimator_predict(result_table, feature_column_names, feature_metas,
                          train_label_name, result_col_name, conn,
                          predict_generator, selected_cols)

    print("Done predicting. Predict table : %s" % result_table)
