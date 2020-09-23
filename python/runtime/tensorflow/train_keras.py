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

import inspect
import sys
import warnings

import six
import tensorflow as tf
from runtime.model import save_metadata
from runtime.tensorflow import metrics
from runtime.tensorflow.get_tf_version import tf_is_version2
from runtime.tensorflow.keras_with_feature_column_input import \
    init_model_with_feature_column
from runtime.tensorflow.load_model import load_keras_model_weights


def keras_compile(estimator, model_params, metric_names):
    # remove optimizer param from model_params and use it when call "compile()"
    optimizer = None
    loss = None
    if "optimizer" in model_params:
        optimizer = model_params["optimizer"]
        del model_params["optimizer"]
    if "loss" in model_params:
        loss = model_params["loss"]
        del model_params["loss"]

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
            keras_metrics = metrics.get_keras_metrics(["Accuracy"])

    # setting optimizer
    has_none_optimizer = False
    if optimizer is None:
        # use keras model default optimizer if optimizer is not specified in
        # WITH clause.
        members = inspect.getmembers(classifier_pkg)
        # default optimizer
        optimizer = tf.keras.optimizers.Adagrad(lr=0.001)
        for m, func in members:
            if m == "optimizer":
                optimizer = classifier_pkg.optimizer()
                if optimizer is None:
                    has_none_optimizer = True
                    warnings.warn('optimizer() returns None')

    if loss is None:
        members = inspect.getmembers(classifier_pkg)
        # FIXME(typhoonzero): default loss may cause error if model's output
        # shape does not fit.
        loss = "sparse_categorical_crossentropy"
        for m, func in members:
            if m == "loss":
                loss = classifier_pkg.loss

    classifier = init_model_with_feature_column(
        estimator, model_params, has_none_optimizer=has_none_optimizer)

    # FIXME(sneaxiy): some models defined by other framework (not TensorFlow or
    # XGBoost) may return None optimizer.
    # For example:
    # https://github.com/sql-machine-learning/models/blob/ce970d14a524e20de10a645c99b6bf8724be17d9/sqlflow_models/arima_with_stl_decomposition.py#L123  # noqa: E501
    if has_none_optimizer:
        assert hasattr(
            classifier,
            "sqlflow_train_loop"), "optimizer() should not return None"
    else:
        classifier.compile(optimizer=optimizer,
                           loss=loss,
                           metrics=keras_metrics)

    return classifier, has_none_optimizer


def keras_train_and_save(estimator, model_params, save, is_pai,
                         train_dataset_fn, val_dataset_fn, label_meta, epochs,
                         verbose, metric_names, validation_steps,
                         load_pretrained_model, model_meta):
    print("Start training using keras model...")
    try:
        classifier, has_none_optimizer = keras_compile(estimator, model_params,
                                                       metric_names)
    except Exception as e:
        if hasattr(estimator, "sqlflow_train_loop"):
            sys.stderr.write(
                "compile keras model failed, ignoring this error "
                "since the model seems to defined sqlflow_train_loop.")
            classifier = init_model_with_feature_column(
                estimator, model_params, has_none_optimizer=True)
            has_none_optimizer = True
        else:
            raise e

    train_dataset = train_dataset_fn()
    if val_dataset_fn is not None:
        validate_dataset = val_dataset_fn()
    else:
        validate_dataset = None

    if load_pretrained_model:
        # Must run one batch to initialize parameters before load_weights
        inputs, targets = next(iter(train_dataset.take(1)))
        classifier.evaluate(inputs, targets)

        # NOTE(sneaxiy): should we save/load optimizer info for incremental
        # training, or let users to write the same WITH statements in SQL?
        load_keras_model_weights(classifier, save)

    keras_train_compiled(classifier, save, train_dataset, validate_dataset,
                         label_meta, epochs, verbose, model_meta,
                         validation_steps, has_none_optimizer)


def keras_train_compiled(classifier, save, train_dataset, validate_dataset,
                         label_meta, epochs, verbose, model_meta,
                         validation_steps, has_none_optimizer):
    if hasattr(classifier, 'sqlflow_train_loop'):
        classifier.sqlflow_train_loop(train_dataset)
    else:
        if label_meta["feature_name"] != "":
            # FIXME(typhoonzero): this is why need to set validation_steps:
            #  https://github.com/tensorflow/tensorflow/issues/29743#issuecomment-502028891
            # remove this argument when PAI fixes this.
            if tf_is_version2():
                validation_steps = None
            else:
                if validate_dataset is None:
                    validation_steps = None
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
        train_metrics = dict()
        val_metrics = dict()
        for k in history.history.keys():
            if k.startswith("val_"):
                val_metrics[k] = float(history.history[k][-1])
            else:
                train_metrics[k] = float(history.history[k][-1])
        print("====== Result for training set: ======")
        for k, v in train_metrics.items():
            print("%s: %s" % (k, v))
        print("====== Result for validation set: ======")
        for k, v in val_metrics.items():
            print("%s: %s" % (k, v))
        model_meta["evaluation"] = val_metrics

    # write model metadata to model_meta.json
    save_metadata("model_meta.json", model_meta)

    try:
        # NOTE: classifier.save may fail if the model has
        # sqlflow_train_loop and does not have Keras layers defined.
        # So save metadata before calling classifier.save.
        classifier.save(save, save_format="tf")
    except:  # noqa: E722
        if has_none_optimizer:
            warnings.warn("Saving model with None optimizer fails")
        else:
            six.reraise(*sys.exc_info())
