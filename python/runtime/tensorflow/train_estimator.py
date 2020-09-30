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

import tensorflow as tf
from runtime.diagnostics import init_model, load_pretrained_model_estimator
from runtime.model import save_metadata
from runtime.seeding import get_tf_random_seed
from runtime.tensorflow.get_tf_version import tf_is_version2
from runtime.tensorflow.metrics import get_tf_metrics


def estimator_train_and_save(estimator, model_params, save, train_dataset_fn,
                             val_dataset_fn, train_max_steps,
                             eval_start_delay_secs, eval_throttle_secs,
                             save_checkpoints_steps, metric_names,
                             load_pretrained_model, model_meta):
    print("Start training using estimator model...")
    model_params["model_dir"] = save
    model_params["config"] = tf.estimator.RunConfig(
        tf_random_seed=get_tf_random_seed(),
        save_checkpoints_steps=save_checkpoints_steps)

    warm_start_from = save if load_pretrained_model else None
    if warm_start_from:
        load_pretrained_model_estimator(estimator, model_params)
    classifier = init_model(estimator, model_params)

    # do not add default Accuracy metric when using estimator to train, it will
    # fail when the estimator is a regressor, and estimator seems automatically
    # add some metrics. Only add additional metrics when user specified with
    # `WITH`.
    if tf_is_version2() and metric_names != ["Accuracy"]:
        classifier = tf.estimator.add_metrics(classifier,
                                              get_tf_metrics(metric_names))

    estimator_train_compiled(classifier, train_dataset_fn, val_dataset_fn,
                             train_max_steps, eval_start_delay_secs,
                             eval_throttle_secs)
    estimator_save(classifier, save, model_params, model_meta)


def estimator_save(classifier, save, model_params, model_meta):
    # export saved model for prediction
    if "feature_columns" in model_params:
        all_feature_columns = model_params["feature_columns"]
    elif "linear_feature_columns" in model_params \
            and "dnn_feature_columns" in model_params:
        import copy
        all_feature_columns = copy.copy(model_params["linear_feature_columns"])
        all_feature_columns.extend(model_params["dnn_feature_columns"])
    else:
        raise Exception("No expected feature columns in model params")
    serving_input_fn = tf.estimator.export.build_parsing_serving_input_receiver_fn(  # noqa: E501
        tf.feature_column.make_parse_example_spec(all_feature_columns))
    export_path = classifier.export_saved_model(save, serving_input_fn)
    # write the path under current directory
    export_path_str = str(export_path.decode("utf-8"))
    with open("exported_path", "w") as fn:
        fn.write(export_path_str)
    # write model metadata to model_meta.json
    save_metadata("model_meta.json", model_meta)
    print("Done training, model exported to: %s" % export_path_str)


def estimator_train_compiled(estimator, train_dataset_fn, val_dataset_fn,
                             train_max_steps, eval_start_delay_secs,
                             eval_throttle_secs):
    if val_dataset_fn is not None:
        train_spec = tf.estimator.TrainSpec(
            input_fn=lambda: train_dataset_fn(), max_steps=train_max_steps)
        eval_spec = tf.estimator.EvalSpec(
            input_fn=lambda: val_dataset_fn(),
            start_delay_secs=eval_start_delay_secs,
            throttle_secs=eval_throttle_secs)
        result = tf.estimator.train_and_evaluate(estimator, train_spec,
                                                 eval_spec)
        if result:
            print(result[0])
    else:
        # NOTE(typhoonzero): if only do training, no validation result will be
        # printed, checkout the training log by setting train.verbose=2.
        estimator.train(lambda: train_dataset_fn(), max_steps=train_max_steps)
