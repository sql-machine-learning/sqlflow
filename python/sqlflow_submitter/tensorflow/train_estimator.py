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

from os import path

import tensorflow as tf

from ..model_metadata import save_model_metadata
from . import metrics
from .diag import init_model, load_pretrained_model_estimator
from .get_tf_version import tf_is_version2
from .input_fn import input_fn
from .pai_distributed import make_estimator_distributed_runconfig


def estimator_train_and_save(estimator, model_params, save, is_pai, FLAGS,
                             train_dataset_fn, val_dataset_fn,
                             log_every_n_iter, train_max_steps,
                             eval_start_delay_secs, eval_throttle_secs,
                             save_checkpoints_steps, metric_names,
                             load_pretrained_model, model_meta):
    print("Start training using estimator model...")

    is_distributed = False
    if is_pai and len(FLAGS.worker_hosts.split(",")) > 1:
        is_distributed = True
    model_params["config"] = make_estimator_distributed_runconfig(
        FLAGS,
        estimator,
        is_distributed,
        save_checkpoints_steps=save_checkpoints_steps)
    if is_pai:
        print("Using checkpoint path: %s" % FLAGS.checkpointDir)
        model_params["model_dir"] = FLAGS.checkpointDir
    else:
        model_params["model_dir"] = save

    warm_start_from = save if load_pretrained_model else None
    if warm_start_from:
        load_pretrained_model_estimator(estimator, model_params)
    classifier = init_model(estimator, model_params)

    # do not add default Accuracy metric when using estimator to train, it will fail
    # when the estimator is a regressor, and estimator seems automatically add some
    # metrics. Only add additional metrics when user specified with `WITH`.
    if tf_is_version2() and metric_names != ["Accuracy"]:
        classifier = tf.estimator.add_metrics(
            classifier, metrics.get_tf_metrics(metric_names))

    estimator_train_compiled(classifier, is_pai, FLAGS, train_dataset_fn,
                             val_dataset_fn, log_every_n_iter, train_max_steps,
                             eval_start_delay_secs, eval_throttle_secs)

    if is_pai and FLAGS.task_index != 0:
        print("skip exporting model on worker != 0")
        return
    # export saved model for prediction
    if "feature_columns" in model_params:
        all_feature_columns = model_params["feature_columns"]
    elif "linear_feature_columns" in model_params and "dnn_feature_columns" in model_params:
        import copy
        all_feature_columns = copy.copy(model_params["linear_feature_columns"])
        all_feature_columns.extend(model_params["dnn_feature_columns"])
    else:
        raise Exception("No expected feature columns in model params")
    serving_input_fn = tf.estimator.export.build_parsing_serving_input_receiver_fn(
        tf.feature_column.make_parse_example_spec(all_feature_columns))
    export_path = classifier.export_saved_model(save, serving_input_fn)
    # write the path under current directory
    export_path_str = str(export_path.decode("utf-8"))
    with open("exported_path", "w") as fn:
        fn.write(export_path_str)
    # write model metadata to model_meta.json
    save_model_metadata(path.join(export_path_str, "model_meta.json"),
                        model_meta)
    print("Done training, model exported to: %s" % export_path_str)


def estimator_train_compiled(estimator, is_pai, FLAGS, train_dataset_fn,
                             val_dataset_fn, log_every_n_iter, train_max_steps,
                             eval_start_delay_secs, eval_throttle_secs):
    if val_dataset_fn != None:
        train_spec = tf.estimator.TrainSpec(
            input_fn=lambda: train_dataset_fn(), max_steps=None)
        eval_spec = tf.estimator.EvalSpec(
            input_fn=lambda: val_dataset_fn(),
            start_delay_secs=eval_start_delay_secs,
            throttle_secs=eval_throttle_secs)
        result = tf.estimator.train_and_evaluate(estimator, train_spec,
                                                 eval_spec)
        # FIXME(typhoonzero): find out why pai will have result == None
        if not is_pai:
            print(result[0])
    else:
        # NOTE(typhoonzero): if only do training, no validation result will be printed.
        estimator.train(lambda: train_dataset_fn(), max_steps=train_max_steps)
