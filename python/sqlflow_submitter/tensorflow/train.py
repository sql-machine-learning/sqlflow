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
try:
    import sqlflow_models
except:
    pass

from sqlflow_submitter.db import connect_with_data_source, db_generator

# Disable Tensorflow INFO and WARNING
import logging
tf.get_logger().setLevel(logging.ERROR)

def get_dtype(type_str):
    if type_str == "float32":
        return tf.float32
    elif type_str == "int64":
        return tf.int64
    else:
        raise TypeError("not supported dtype: %s" % type_str)

def parse_sparse_feature(features, label, feature_column_names, feature_metas):
    features_dict = dict()
    for idx, col in enumerate(features):
        name = feature_column_names[idx]
        if feature_metas[name]["is_sparse"]:
            i, v, s = col
            features_dict[name] = tf.SparseTensor(indices=i, values=v, dense_shape=s)
        else:
            features_dict[name] = col
    return features_dict, label


def train(is_keras_model,
          datasource,
          estimator,
          select,
          validate_select,
          feature_columns,
          feature_column_names,
          feature_metas={},
          label_meta={},
          model_params={},
          save="",
          batch_size=1,
          epochs=1,
          verbose=0):
    conn = connect_with_data_source(datasource)
    if not is_keras_model:
        classifier = estimator(**feature_columns, **model_params, model_dir=save)
    else:
        classifier = estimator(**feature_columns, **model_params)

    def input_fn(datasetStr):
        feature_types = []
        for name in feature_column_names:
            # NOTE: vector columns like 23,21,3,2,0,0 should use shape None
            if feature_metas[name]["is_sparse"]:
                feature_types.append((tf.int64, tf.int32, tf.int64))
            else:
                feature_types.append(get_dtype(feature_metas[name]["dtype"]))

        gen = db_generator(conn.driver, conn, datasetStr, feature_column_names, label_meta["feature_name"], feature_metas)
        dataset = tf.data.Dataset.from_generator(gen, (tuple(feature_types), eval("tf.%s" % label_meta["dtype"])))
        ds_mapper = functools.partial(parse_sparse_feature, feature_column_names=feature_column_names, feature_metas=feature_metas)
        return dataset.map(ds_mapper)

    def train_input_fn(batch_size):
        dataset = input_fn(select)
        dataset = dataset.shuffle(1000).batch(batch_size)
        if not is_keras_model:
            dataset = dataset.repeat(epochs if epochs else 1)
        
        return dataset

    def validate_input_fn(batch_size):
        dataset = input_fn(validate_select)
        return dataset.batch(batch_size).cache(filename="dataset_cache_val.txt")

    if is_keras_model:
        classifier.compile(optimizer=classifier.default_optimizer(),
            loss=classifier.default_loss(),
            metrics=["accuracy"])
        if hasattr(classifier, 'sqlflow_train_loop'):
            # NOTE(typhoonzero): do not cache dataset if using sqlflow_train_loop, it may use the dataset multiple times causing "tensorflow.python.framework.errors_impl.AlreadyExistsError":
            # https://github.com/sql-machine-learning/models/blob/a3559618a013820385f43307261ad34351da2fbf/sqlflow_models/deep_embedding_cluster.py#L126
            classifier.sqlflow_train_loop(train_input_fn(batch_size))
        else:
            ds = train_input_fn(batch_size).cache(filename="dataset_cache_train.txt")
            classifier.fit(ds,
                epochs=epochs if epochs else classifier.default_training_epochs(),
                verbose=verbose)
        classifier.save_weights(save, save_format="h5")
        if label_meta["feature_name"] != "" and validate_select != "":
            eval_result = classifier.evaluate(validate_input_fn(batch_size), verbose=verbose)
            print("Training set accuracy: {accuracy:0.5f}".format(**{"accuracy": eval_result[1]}))
    else:
        classifier.train(input_fn=lambda:train_input_fn(batch_size))
        if validate_select != "":
            eval_result = classifier.evaluate(input_fn=lambda:validate_input_fn(batch_size))
            print("Evaluation result:", eval_result)

    print("Done training")
