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

import sys

import tensorflow as tf
from sqlflow_submitter.pai import model

from . import metrics
from .get_tf_version import tf_is_version2
from .input_fn import input_fn


def keras_train_and_save(estimator, model_params, save, is_pai, FLAGS,
                         train_dataset_fn, val_dataset_fn, label_meta, epochs,
                         verbose, metric_names, validation_steps):
    print("Start training using keras model...")
    # remove optimizer param from model_params and use it when call "compile()"
    optimizer = None
    loss = None
    if "optimizer" in model_params:
        optimizer = model_params["optimizer"]
        del model_params["optimizer"]
    if "loss" in model_params:
        loss = model_params["loss"]
        del model_params["loss"]

    classifier = estimator(**model_params)
    classifier_pkg = sys.modules[estimator.__module__]
    model_metrics = []
    if hasattr(classifier_pkg, "eval_metrics_fn"):
        metrics_functions = classifier_pkg.eval_metrics_fn()
        for key, func in metrics_functions.items():
            func.__name__ = key
            model_metrics.append(func)
    # use WITH specified metrics if it's not default.
    if metric_names != ["Accuracy"]:
        keras_metrics = metrics.get_keras_metrics(metric_names)
    else:
        if len(model_metrics) > 0:
            keras_metrics = model_metrics
        else:
            # default
            keras_metrics = metrics.get_keras_metrics(["Accuracy"])

    train_dataset = train_dataset_fn()
    if val_dataset_fn != None:
        validate_dataset = val_dataset_fn()
    else:
        validate_dataset = None

    if optimizer is None:
        # use keras model default optimizer if optimizer is not specified in WITH clause.
        optimizer = classifier_pkg.optimizer()
    if loss is None:
        loss = classifier_pkg.loss
    classifier.compile(optimizer=optimizer, loss=loss, metrics=keras_metrics)
    if hasattr(classifier, 'sqlflow_train_loop'):

        def flatten(feature, label):
            # TODO(shendiaomo): Modify the cluster model to adapt the new input structure
            for k in feature:
                feature[k] = feature[k][0]
            return feature, [label]

        def flatten_feature_only(feature):
            for k in feature:
                feature[k] = feature[k][0]
            return feature

        if label_meta["feature_name"] == "":
            # Clustering model do not have label
            train_dataset = train_dataset.map(flatten_feature_only)
        else:
            train_dataset = train_dataset.map(flatten)

        classifier.sqlflow_train_loop(train_dataset)
    else:
        if label_meta["feature_name"] != "":
            # FIXME(typhoonzero): this is why need to set validation_steps: https://github.com/tensorflow/tensorflow/issues/29743#issuecomment-502028891
            # remove this argument when PAI fixes this.
            if tf_is_version2():
                validation_steps = None
            else:
                validation_steps = 1
            history = classifier.fit(train_dataset,
                                     validation_steps=validation_steps,
                                     epochs=epochs if epochs else
                                     classifier.default_training_epochs(),
                                     validation_data=validate_dataset,
                                     verbose=verbose)
        else:
            history = classifier.fit(train_dataset,
                                     validation_steps=validation_steps,
                                     epochs=epochs if epochs else
                                     classifier.default_training_epochs(),
                                     verbose=verbose)
        train_keys = []
        val_keys = []
        for k in history.history.keys():
            if k.startswith("val_"):
                val_keys.append(k)
            else:
                train_keys.append(k)
        print("====== Result for training set: ======")
        for k in train_keys:
            print("%s: %s" % (k, history.history[k][-1]))
        print("====== Result for validation set: ======")
        for k in val_keys:
            print("%s: %s" % (k, history.history[k][-1]))
    classifier.save_weights(save, save_format="h5")
    if is_pai:
        print("saving keras model to: %s" % FLAGS.sqlflow_oss_modeldir)
        model.save_file(FLAGS.sqlflow_oss_modeldir, save)
