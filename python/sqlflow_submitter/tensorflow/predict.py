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
from sqlflow_submitter.tensorflow.train import get_dtype, parse_sparse_feature

TF_VERSION_2 = True  # TODO(shendiaomo): Remove after we fully upgrade to TF2.0
# Disable Tensorflow INFO and WARNING
try:
    if tf.version.VERSION > '1':
        import logging
        tf.get_logger().setLevel(logging.ERROR)
    else:
        raise ImportError
except:
    tf.logging.set_verbosity(tf.logging.ERROR)
    TF_VERSION_2 = False

FLAGS = None
# ----------------- For PAI predicting -----------------
def define_tf_flags():
    global FLAGS
    if not TF_VERSION_2:
        tf.app.flags.DEFINE_string("checkpointDir", "", "oss info")
        FLAGS = tf.app.flags.FLAGS

class FastPredict:
    def __init__(self, estimator, input_fn):
        self.estimator = estimator
        self.input_fn = input_fn

    def predict(self, feature_and_label):
        def inner_func():
            # FIXME(tony): don't yield label
            feature, label = feature_and_label[0], feature_and_label[1]
            yield feature, label
        predictions = self.estimator.predict(input_fn=self.input_fn(inner_func))
        return [n for n in predictions]

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
    global FLAGS
    define_tf_flags()
    if not is_pai:
        conn = connect_with_data_source(datasource)
    model_params.update(feature_columns)

    if is_keras_model:
        if not issubclass(estimator, tf.keras.Model):
            # functional model need field_metas parameter
            model_params["field_metas"] = feature_metas
        classifier = estimator(**model_params)
        classifier_pkg = sys.modules[estimator.__module__]

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
        one_batch = pred_dataset.__iter__().next()
        # NOTE: must run predict one batch to initialize parameters
        # see: https://www.tensorflow.org/alpha/guide/keras/saving_and_serializing#saving_subclassed_models
        classifier.predict_on_batch(one_batch[0])
        classifier.load_weights(save)
        del pred_dataset
        pred_dataset = eval_input_fn(1, cache=True).make_one_shot_iterator()
        buff_rows = []
        column_names = feature_column_names[:]
        column_names.append(label_meta["feature_name"])
        with buffered_db_writer(conn.driver, conn, result_table, column_names, 100, hdfs_namenode_addr, hive_location, hdfs_user, hdfs_pass) as w:
            while True:
                try:
                    features = pred_dataset.get_next()
                except tf.errors.OutOfRangeError:
                    break
                result = classifier.predict_on_batch(features[0])
                result = classifier_pkg.prepare_prediction_column(result[0])
                row = []
                for idx, name in enumerate(feature_column_names):
                    val = features[0][name].numpy()[0]
                    row.append(str(val))
                row.append(str(result))
                w.write(row)
        del pred_dataset
    else:
        if is_pai:
            model_params["model_dir"] = FLAGS.checkpointDir
        else:
            model_params['model_dir'] = save
        classifier = estimator(**model_params)

        # FIXME(typhoonzero): copied from train.py
        def pai_maxcompute_input_fn():
            table_parts = pai_table.split(".")
            if len(table_parts) == 2:
                database, table_name = table_parts
            elif len(table_parts) == 1:
                table_name = pai_table
                driver, dsn = datasource.split("://")
                database = parseMaxComputeDSN(dsn)[-1]
            else:
                raise ValueError("error database.table format: %s" % pai_table)

            tables = ["odps://%s/tables/%s" % (database, table_name)]
            record_defaults = []
            for name in feature_column_names:
                dtype = get_dtype(feature_metas[name]["dtype"])
                record_defaults.append(tf.constant(0, dtype=dtype, shape=feature_metas[name]["shape"]))

            dataset = tf.data.TableRecordDataset(tables,
                                        record_defaults=record_defaults,
                                        selected_cols=",".join(feature_column_names))
            def tensor_to_dict(*args):
                num_features = len(feature_column_names)
                features_dict = dict()
                for idx in range(num_features):
                    name = feature_column_names[idx]
                    features_dict[name] = tf.reshape(args[idx], [-1])
                return features_dict

            return dataset.map(tensor_to_dict)

        def fast_input_fn(generator):
            feature_types = []
            for name in feature_column_names:
                if feature_metas[name]["is_sparse"]:
                    feature_types.append((tf.int64, tf.int32, tf.int64))
                else:
                    feature_types.append(get_dtype(feature_metas[name]["dtype"]))

            def _inner_input_fn():
                if is_pai:
                    dataset = pai_maxcompute_input_fn()
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
        pred_gen = db_generator(conn.driver, conn, select, feature_column_names, label_meta["feature_name"], feature_metas)()
        fast_predictor = FastPredict(classifier, fast_input_fn)

        with buffered_db_writer(conn.driver, conn, result_table, column_names, 100, hdfs_namenode_addr, hive_location, hdfs_user, hdfs_pass) as w:
            while True:
                try:
                    features = next(pred_gen)
                except StopIteration:
                    break
                result = fast_predictor.predict(features)
                row = []
                for idx, _ in enumerate(feature_column_names):
                    val = features[0][idx]
                    row.append(str(val))
                if "class_ids" in list(result)[0]:
                    row.append(str(list(result)[0]["class_ids"][0]))
                else:
                    # regression predictions
                    row.append(str(list(result)[0]["predictions"][0]))
                w.write(row)

    print("Done predicting. Predict table : %s" % result_table)
