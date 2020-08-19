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

import runtime.pai.pai_distributed as pai_dist
import six
import xgboost as xgb
from runtime.local.xgboost_submitter.save import save_model_to_local_file
from runtime.model import collect_metadata
from runtime.model import oss as pai_model_store
from runtime.model import save_metadata
from runtime.xgboost.dataset import xgb_dataset
from runtime.xgboost.pai_rabit import PaiXGBoostTracker, PaiXGBoostWorker


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
               model_repo_image="",
               original_sql=""):
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
                  model_repo_image=model_repo_image,
                  original_sql=original_sql)
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
          model_repo_image="",
          original_sql=""):
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

    re = None
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
        # TODO(sneaxiy): collect features and label
        metadata = collect_metadata(original_sql=original_sql,
                                    select=select,
                                    validation_select=validation_select,
                                    model_repo_image=model_repo_image,
                                    class_name=model_params.get("booster"),
                                    attributes=model_params,
                                    features=None,
                                    label=None,
                                    evaluation=re)
        save_model_to_local_file(bst, model_params, filename)
        save_metadata("model_meta.json", metadata)
        if is_pai and len(oss_model_dir) > 0:
            save_model(oss_model_dir, filename, model_params, train_params,
                       feature_metas, feature_column_names, label_meta,
                       feature_column_code)


def save_model(model_dir, filename, model_params, train_params, feature_metas,
               feature_column_names, label_meta, feature_column_code):
    pai_model_store.save_file(model_dir, filename)
    pai_model_store.save_file(model_dir, "{}.pmml".format(filename))
    pai_model_store.save_file(model_dir, "model_meta.json")
    # (TODO:lhw) remove this function call, use the new metadata in load_metas
    pai_model_store.save_metas(
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
