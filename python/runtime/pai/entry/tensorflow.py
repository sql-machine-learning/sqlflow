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

from runtime import oss
from runtime.pai.pai_distributed import define_tf_flags, set_oss_environs
from runtime.tensorflow import explain, is_tf_estimator, predict, train
from runtime.tensorflow.diag import SQLFlowDiagnostic


def load_oss_model(params, oss_model_dir, estimator):
    conf = params["conf"]
    attributes = params["model_params"]
    if conf["worker"]["count"] > 1 and "validation.select" not in attributes:
        raise SQLFlowDiagnostic(
            "Distributed training must specify WITH validation.select")

    is_estimator = is_tf_estimator(estimator)
    # Keras single node is using h5 format to save the model, no need to deal with export model format.
    # Keras distributed mode will use estimator, so this is also needed.
    if is_estimator:
        oss.load_file(oss_model_dir, "exported_path")
        # NOTE(typhoonzero): directory "model_save" is hardcoded in codegen/tensorflow/codegen.go
        oss.load_dir(oss_model_dir + "/model_save")
    else:
        oss.load_file(oss_model_dir, "model_save")


def do_train(params):
    train.train(params["datasource"], params["estimator_string"],
                params["select"], params["validation_select"],
                params["feature_columns"], params["feature_column_names"],
                **params)
    save_oss_model(params["oss_model_dir"], params["estimator"],
                   params["feature_column_names"],
                   params["feature_column_name_map"], params["feature_metas"],
                   params["label_meta"], params["model_params"],
                   params["feature_columns_code"])


def save_oss_model(oss_model_dir, estimator, feature_column_names,
                   feature_column_names_map, feature_metas, label_meta,
                   model_params, feature_columns_code):
    is_estimator = is_tf_estimator(estimator)
    num_workers = len(FLAGS.worker_hosts.split(","))

    # Keras single node is using h5 format to save the model, no need to deal with export model format.
    # Keras distributed mode will use estimator, so this is also needed.
    if is_estimator:
        if FLAGS.task_index == 0:
            with open("exported_path", "r") as fn:
                saved_model_path = fn.read()
            oss.save_dir(oss_model_dir, saved_model_path)
            oss.save_file(oss_model_dir, "exported_path")
    else:
        if len(FLAGS.worker_hosts.split(",")) > 1:
            if FLAGS.task_index == 0:
                oss.save_file(oss_model_dir, "exported_path")
        else:
            oss.save_file(oss_model_dir, "model_save")

    oss.save_metas(oss_model_dir, num_workers, "tensorflow_model_desc",
                   estimator, feature_column_names, feature_column_names_map,
                   feature_metas, label_meta, model_params,
                   feature_columns_code)


def do_predict(params):
    predict.pred(params["datasource"], params["estimator"], params["select"],
                 params["result_table"], params["feature_columns"],
                 params["feature_column_names"],
                 params["feature_column_names_map"], params["result_col_name"],
                 **params)


def do_explain(params):
    explain.explain(params["datasource"], params["estimator"],
                    params["select"], params["feature_columns"],
                    params["feature_column_names"], **params)


def entrypoint():
    with open("train_params.pkl", "r") as file:
        params = pickle.load(file)
    if params["entry_type"] == "train":
        do_train(params)
    elif params["entry_type"] == "predict":
        do_predict(params)
    elif params["entry_type"] == "explain":
        do_explain(params)


if __name__ == "__main__":
    FLAGS = define_tf_flags()
    set_oss_environs(FLAGS)
    entrypoint()
