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
from os import path

import six
import tensorflow as tf
from sqlflow_submitter.pai import model
from sqlflow_submitter.seeding import get_tf_random_seed

from ..model_metadata import save_model_metadata
from . import metrics
from .get_tf_version import tf_is_version2
from .input_fn import input_fn
from .keras_with_feature_column_input import init_model_with_feature_column
from .pai_distributed import (dump_into_tf_config,
                              make_distributed_info_without_evaluator)
from .train_estimator import estimator_train_compiled


def keras_train_and_save(estimator, model_params, save, is_pai, FLAGS,
                         train_dataset_fn, val_dataset_fn, label_meta, epochs,
                         verbose, metric_names, validation_steps,
                         load_pretrained_model, model_meta):
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

    classifier_pkg = sys.modules[estimator.__module__]
    # setting training metrics
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

    # setting optimizer
    has_none_optimizer = False
    if optimizer is None:
        # use keras model default optimizer if optimizer is not specified in WITH clause.
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
        # FIXME(typhoonzero): default loss may cause error if model's output shape does not fit.
        loss = "sparse_categorical_crossentropy"
        for m, func in members:
            if m == "loss":
                loss = classifier_pkg.loss

    # setting datasets
    train_dataset = train_dataset_fn()
    if val_dataset_fn != None:
        validate_dataset = val_dataset_fn()
    else:
        validate_dataset = None

    classifier = init_model_with_feature_column(
        estimator, model_params, has_none_optimizer=has_none_optimizer)

    # FIXME(sneaxiy): some models defined by other framework (not TensorFlow or XGBoost)
    # may return None optimizer.
    # For example: https://github.com/sql-machine-learning/models/blob/ce970d14a524e20de10a645c99b6bf8724be17d9/sqlflow_models/arima_with_stl_decomposition.py#L123
    if has_none_optimizer:
        assert hasattr(
            classifier,
            "sqlflow_train_loop"), "optimizer() should not return None"
    else:
        classifier.compile(optimizer=optimizer,
                           loss=loss,
                           metrics=keras_metrics)

    if load_pretrained_model:
        # Must run one batch to initialize parameters before load_weights
        inputs, targets = next(iter(train_dataset.take(1)))
        classifier.evaluate(inputs, targets)

        # NOTE(sneaxiy): should we save/load optimizer info for incremental training, or
        # let users to write the same WITH statements in SQL?
        classifier.load_weights(save)

    if is_pai and len(FLAGS.worker_hosts.split(",")) > 1:
        # train keras model distributed
        cluster, task_type, task_index = make_distributed_info_without_evaluator(
            FLAGS)
        dump_into_tf_config(cluster, task_type, task_index)
        dist_strategy = tf.contrib.distribute.ParameterServerStrategy()

        run_config = tf.estimator.RunConfig(
            tf_random_seed=get_tf_random_seed(),
            save_checkpoints_steps=100,
            train_distribute=dist_strategy,
            session_config=tf.ConfigProto(log_device_placement=True,
                                          device_filters=None))
        model_dir = FLAGS.checkpointDir

        keras_estimator = tf.keras.estimator.model_to_estimator(
            classifier, model_dir=model_dir, config=run_config)
        estimator_train_compiled(
            keras_estimator,
            is_pai,
            FLAGS,
            train_dataset_fn,
            val_dataset_fn,
            # TODO(typhoonzero): do pass train settings.
            100,
            None,
            60,
            120)
        # FIXME(typhoonzero): predict keras distributed model should also call model_to_estimator.
        # export saved model for prediction
        if "feature_columns" in model_params:
            all_feature_columns = model_params["feature_columns"]
        elif "linear_feature_columns" in model_params and "dnn_feature_columns" in model_params:
            import copy
            all_feature_columns = copy.copy(
                model_params["linear_feature_columns"])
            all_feature_columns.extend(model_params["dnn_feature_columns"])
        else:
            raise Exception("No expected feature columns in model params")
        serving_input_fn = tf.estimator.export.build_parsing_serving_input_receiver_fn(
            tf.feature_column.make_parse_example_spec(all_feature_columns))
        export_path = keras_estimator.export_saved_model(
            save, serving_input_fn)

        # write the path under current directory
        export_path_str = str(export_path.decode("utf-8"))
        with open("exported_path", "w") as fn:
            fn.write(export_path_str)
        # write model metadata to model_meta.json
        save_model_metadata(path.join(export_path_str, "model_meta.json"),
                            model_meta)
        print("Done training, model exported to: %s" % export_path_str)
        return

    if hasattr(classifier, 'sqlflow_train_loop'):
        classifier.sqlflow_train_loop(train_dataset)
    else:
        if label_meta["feature_name"] != "":
            # FIXME(typhoonzero): this is why need to set validation_steps: https://github.com/tensorflow/tensorflow/issues/29743#issuecomment-502028891
            # remove this argument when PAI fixes this.
            if tf_is_version2():
                validation_steps = None
            else:
                if validate_dataset == None:
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

    try:
        classifier.save_weights(save, save_format="h5")
        # write model metadata to model_meta.json
        save_model_metadata("model_meta.json", model_meta)
        if is_pai:
            print("saving keras model to: %s" % FLAGS.sqlflow_oss_modeldir)
            model.save_file(FLAGS.sqlflow_oss_modeldir, save)
            model.save_file(FLAGS.sqlflow_oss_modeldir, "model_meta.json")
    except:
        if has_none_optimizer:
            warnings.warn("Saving model with None optimizer fails")
        else:
            six.reraise(*sys.exc_info())
