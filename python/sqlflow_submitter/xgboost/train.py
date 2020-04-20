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

import os
# TODO(weiguoz) remove me
import sys
import time

import sqlflow_submitter.tensorflow.pai_distributed as pai_dist
import xgboost as xgb
from sqlflow_submitter.xgboost.dataset import xgb_dataset
from sqlflow_submitter.xgboost.pai_rabit import PaiTracker, PaiWorker


def dist_train(flags,
               num_workers,
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
               is_pai=False,
               pai_train_table="",
               pai_validate_table=""):
    num_hosts = len(flags.worker_hosts.split(","))
    if not is_pai or num_workers <= 1 or num_hosts != num_workers:
        raise Exception(
            "dist xgb train is supported for pai with #workers > 1")

    cluster, node, task_id = pai_dist.make_distributed_info_without_evaluator(
        flags)
    master_addr = cluster["ps"][0].split(":")
    master_host = master_addr[0]
    master_port = int(master_addr[1]) + 1
    tracker = None
    print("node={}\ttask_id={}\tcluster={}".format(node, task_id, cluster))
    try:
        if node == 'ps':
            tracker = PaiTracker(host=master_host,
                                 nworkers=num_workers,
                                 port=master_port)
        else:
            if node != 'chief':
                task_id += 1
            envs = PaiWorker.gen_envs(host=master_host,
                                      port=master_port,
                                      ttl=200,
                                      nworkers=num_workers,
                                      task_id=task_id)
            xgb.rabit.init(envs)
            rank = xgb.rabit.get_rank()

            train(datasource, select, model_params, train_params,
                  feature_metas, feature_column_names, label_meta,
                  validation_select, disk_cache, batch_size, epoch, is_pai,
                  pai_train_table, pai_validate_table)
            print("[%s]train done" % time.strftime("%Y-%m-%d %H:%M:%S"))
            if rank == 0:
                print("[%s]I'm going to save the model" %
                      time.strftime("%Y-%m-%d %H:%M:%S"))
    except Exception as e:
        raise e
    finally:
        if tracker is not None:
            tracker.join()
        if node != 'ps':
            xgb.rabit.finalize()
        # FIXME(weiguoz)
        sys.exit(0)


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
          is_pai=False,
          pai_train_table="",
          pai_validate_table=""):
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
                         epoch=epoch)
    bst = None
    for per_batch_dmatrix in dtrain:
        watchlist = [(per_batch_dmatrix, "train")]
        if len(validation_select.strip()) > 0:
            dvalidate = list(
                xgb_dataset(datasource, 'validate.txt', validation_select,
                            feature_metas, feature_column_names, label_meta,
                            is_pai, pai_validate_table))[0]
            watchlist.append((dvalidate, "validate"))

        re = dict()
        bst = xgb.train(model_params,
                        per_batch_dmatrix,
                        evals=watchlist,
                        evals_result=re,
                        xgb_model=bst,
                        **train_params)
        bst.save_model("my_model")
        print("Evaluation result: %s" % re)
