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

import pickle
import types

from sqlflow_submitter.tensorflow import is_tf_estimator, train
from sqlflow_submitter.tensorflow.pai_distributed import (define_tf_flags,
                                                          set_oss_environs)

from .. import model

FLAGS = define_tf_flags()


def load_oss_model(oss_model_dir, estimator):
    set_oss_environs(FLAGS)

    is_estimator = is_tf_estimator(estimator)

    # Keras single node is using h5 format to save the model, no need to deal with export model format.
    # Keras distributed mode will use estimator, so this is also needed.
    if is_estimator:
        model.load_file(oss_model_dir, "exported_path")
        # NOTE(typhoonzero): directory "model_save" is hardcoded in codegen/tensorflow/codegen.go
        model.load_dir(oss_model_dir + "/model_save")
    else:
        model.load_file(oss_model_dir, "model_save")


def do_train(params):
    train.train(params["datasource"], params["estimator_string"],
                params["select"], params["validation_select"],
                params["feature_columns"], params["feature_column_names"],
                **params)
    # (TODO: lhw) save model to OSS


def save_oss_model(oss_model_dir, estimator, num_workers, feature_column_names,
                   feature_column_names_map, feature_metas, label_meta,
                   model_params, feature_columns_code):
    is_estimator = is_tf_estimator(estimator)

    # Keras single node is using h5 format to save the model, no need to deal with export model format.
    # Keras distributed mode will use estimator, so this is also needed.
    if is_estimator:
        if FLAGS.task_index == 0:
            with open("exported_path", "r") as fn:
                saved_model_path = fn.read()
            model.save_dir(oss_model_dir, saved_model_path)
            model.save_file(oss_model_dir, "exported_path")
    else:
        if len(FLAGS.worker_hosts.split(",")) > 1:
            if FLAGS.task_index == 0:
                model.save_file(oss_model_dir, "exported_path")
        else:
            model.save_file(oss_model_dir, "model_save")

    model.save_metas(oss_model_dir, num_workers, "tensorflow_model_desc",
                     estimator, feature_column_names, feature_column_names_map,
                     feature_metas, label_meta, model_params,
                     feature_columns_code)


def entry_point():
    with open("train_params.pkl", "r") as file:
        params = pickle.load(file)
    if params["entry_type"] == "train":
        do_train(params)


if __name__ == "__main__":
    entry_point()
