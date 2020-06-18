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

import json
import os
import sys

import six
import sqlflow_submitter.tensorflow.pai_distributed as pai_dist
import xgboost as xgb
from sqlflow_submitter.pai import model
from sqlflow_submitter.xgboost.dataset import xgb_dataset
from sqlflow_submitter.xgboost.pai_rabit import (PaiXGBoostTracker,
                                                 PaiXGBoostWorker)

from ..model_metadata import collect_model_metadata, save_model_metadata


def dist_train(flags,
               datasource,
               select,
               model_params,
               train_params,
               feature_metas,
               feature_column_names,
               label_meta,
               validation_select,
               disk_cache=False,
               batch_size=None,
               epoch=1,
               load_pretrained_model=False,
               is_pai=False,
               pai_train_table="",
               pai_validate_table="",
               oss_model_dir="",
               transform_fn=None,
               feature_column_code="",
               model_repo_image=""):
    if not is_pai:
        raise Exception(
            "XGBoost distributed training is only supported on PAI")

    num_workers = len(flags.worker_hosts.split(","))
    cluster, node, task_id = pai_dist.make_distributed_info_without_evaluator(
        flags)
    master_addr = cluster["ps"][0].split(":")
    master_host = master_addr[0]
    master_port = int(master_addr[1]) + 1
    tracker = None
    print("node={}, task_id={}, cluster={}".format(node, task_id, cluster))
    try:
        if node == 'ps':
            if task_id == 0:
                tracker = PaiXGBoostTracker(host=master_host,
                                            nworkers=num_workers,
                                            port=master_port)
        else:
            if node != 'chief':
                task_id += 1
            envs = PaiXGBoostWorker.gen_envs(host=master_host,
                                             port=master_port,
                                             ttl=200,
                                             nworkers=num_workers,
                                             task_id=task_id)
            xgb.rabit.init(envs)
            rank = xgb.rabit.get_rank()

            train(datasource,
                  select,
                  model_params,
                  train_params,
                  feature_metas,
                  feature_column_names,
                  label_meta,
                  validation_select,
                  disk_cache,
                  batch_size,
                  epoch,
                  load_pretrained_model,
                  is_pai,
                  pai_train_table,
                  pai_validate_table,
                  rank,
                  nworkers=num_workers,
                  oss_model_dir=oss_model_dir,
                  transform_fn=transform_fn,
                  feature_column_code=feature_column_code,
                  model_repo_image=model_repo_image)
    except Exception as e:
        print("node={}, id={}, exception={}".format(node, task_id, e))
        six.reraise(*sys.exc_info())  # For better backtrace
    finally:
        if tracker is not None:
            tracker.join()
        if node != 'ps':
            xgb.rabit.finalize()


def train(datasource,
          select,
          model_params,
          train_params,
          feature_metas,
          feature_column_names,
          label_meta,
          validation_select,
          disk_cache=False,
          batch_size=None,
          epoch=1,
          load_pretrained_model=False,
          is_pai=False,
          pai_train_table="",
          pai_validate_table="",
          rank=0,
          nworkers=1,
          oss_model_dir="",
          transform_fn=None,
          feature_column_code="",
          model_repo_image=""):
    if batch_size == -1:
        batch_size = None
    print("Start training XGBoost model...")
    dtrain = xgb_dataset(datasource,
                         'train.txt',
                         select,
                         feature_metas,
                         feature_column_names,
                         label_meta,
                         is_pai,
                         pai_train_table,
                         cache=disk_cache,
                         batch_size=batch_size,
                         epoch=epoch,
                         rank=rank,
                         nworkers=nworkers,
                         transform_fn=transform_fn,
                         feature_column_code=feature_column_code)
    if len(validation_select.strip()) > 0:
        dvalidate = list(
            xgb_dataset(datasource,
                        'validate.txt',
                        validation_select,
                        feature_metas,
                        feature_column_names,
                        label_meta,
                        is_pai,
                        pai_validate_table,
                        rank=rank,
                        nworkers=nworkers,
                        transform_fn=transform_fn,
                        feature_column_code=feature_column_code))[0]

    filename = "my_model"
    if load_pretrained_model:
        bst = xgb.Booster()
        bst.load_model(filename)
    else:
        bst = None

    for per_batch_dmatrix in dtrain:
        watchlist = [(per_batch_dmatrix, "train")]
        if len(validation_select.strip()) > 0:
            watchlist.append((dvalidate, "validate"))

        re = dict()
        bst = xgb.train(model_params,
                        per_batch_dmatrix,
                        evals=watchlist,
                        evals_result=re,
                        xgb_model=bst,
                        **train_params)
        print("Evaluation result: %s" % re)

    if rank == 0:
        metadata = collect_model_metadata(select, validation_select, None,
                                          model_params, train_params,
                                          feature_metas, label_meta, re,
                                          model_repo_image)
        save_model_to_local_file(bst, model_params, metadata, filename)

        if is_pai and len(oss_model_dir) > 0:
            save_model(oss_model_dir, filename, model_params, train_params,
                       feature_metas, feature_column_names, label_meta,
                       feature_column_code)


def save_model_to_local_file(booster, model_params, meta, filename):
    from sklearn2pmml import PMMLPipeline, sklearn2pmml
    try:
        from xgboost.compat import XGBoostLabelEncoder
    except:
        # xgboost==0.82.0 does not have XGBoostLabelEncoder in xgboost.compat.py
        from xgboost.sklearn import XGBLabelEncoder as XGBoostLabelEncoder

    objective = model_params.get("objective")
    bst_meta = dict()

    if objective.startswith("binary:") or objective.startswith("multi:"):
        if objective.startswith("binary:"):
            num_class = 2
        else:
            num_class = model_params.get("num_class")
            assert num_class is not None and num_class > 0, "num_class should not be None"

        # To fake a trained XGBClassifier, there must be "_le", "classes_", inside
        # XGBClassifier. See here:
        # https://github.com/dmlc/xgboost/blob/d19cec70f1b40ea1e1a35101ca22e46dd4e4eecd/python-package/xgboost/sklearn.py#L356
        model = xgb.XGBClassifier()
        label_encoder = XGBoostLabelEncoder()
        label_encoder.fit(list(range(num_class)))
        model._le = label_encoder
        model.classes_ = model._le.classes_

        bst_meta["_le"] = {"classes_": model.classes_.tolist()}
        bst_meta["classes_"] = model.classes_.tolist()
    elif objective.startswith("reg:"):
        model = xgb.XGBRegressor()
    elif objective.startswith("rank:"):
        model = xgb.XGBRanker()
    else:
        raise ValueError(
            "Not supported objective {} for saving PMML".format(objective))

    model_type = type(model).__name__
    bst_meta["type"] = model_type

    # Meta data is needed for saving sklearn pipeline. See here:
    # https://github.com/dmlc/xgboost/blob/d19cec70f1b40ea1e1a35101ca22e46dd4e4eecd/python-package/xgboost/sklearn.py#L356
    booster.set_attr(scikit_learn=json.dumps(bst_meta))
    booster.save_model(filename)
    save_model_metadata("model_meta.json", meta)
    booster.set_attr(scikit_learn=None)
    model.load_model(filename)

    pipeline = PMMLPipeline([(model_type, model)])
    sklearn2pmml(pipeline, "{}.pmml".format(filename))


def save_model(model_dir, filename, model_params, train_params, feature_metas,
               feature_column_names, label_meta, feature_column_code):
    model.save_file(model_dir, filename)
    model.save_file(model_dir, "{}.pmml".format(filename))
    model.save_file(model_dir, "model_meta.json")
    # (TODO:lhw) remove this function call, use the new metadata in load_metas
    model.save_metas(
        model_dir,
        1,
        "xgboost_model_desc",
        "",  # estimator = ""
        model_params,
        train_params,
        feature_metas,
        feature_column_names,
        label_meta,
        feature_column_code)
