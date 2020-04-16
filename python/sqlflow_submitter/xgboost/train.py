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
from sqlflow_submitter.xgboost.tracker import RabitTracker


def trace_port(port):
    cmd = 'lsof -i:%d' % port
    stream = os.popen(cmd)
    output = stream.read()
    print(cmd)
    print("=>")
    print(output)

    cmd = 'for pid in $(lsof -ntP -i:%d); do ps -ef|grep $pid; done' % port
    stream = os.popen(cmd)
    output = stream.read()
    print(cmd)
    print("=>")
    print(output)


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
    rabit_tracker = None
    print("[%s]+++++++++++++++" % time.strftime("%Y-%m-%d %H:%M:%S"))
    print(flags.worker_hosts)
    print(node)
    print(task_id)
    print(cluster)
    try:
        if node == 'ps':
            print("[%s]I'm the master" % time.strftime("%Y-%m-%d %H:%M:%S"))
            trace_port(master_port)
            rabit = RabitTracker(hostIP=master_host,
                                 nslave=num_workers,
                                 port_start=master_port,
                                 port_end=master_port + 1)
            print("[%s]going to start the master" %
                  time.strftime("%Y-%m-%d %H:%M:%S"))
            rabit.start(num_workers)
            rabit_tracker = rabit
            print("[%s]master started" % time.strftime("%Y-%m-%d %H:%M:%S"))
        else:
            envs = [
                'DMLC_NUM_WORKER=%d' % (num_workers),
                'DMLC_TRACKER_URI=%s' % master_host,
                'DMLC_TRACKER_PORT=%d' % master_port,
                'DMLC_TASK_ID=%d' %
                (task_id if node == 'chief' else task_id + 1)
            ]
            for i, env in enumerate(envs):
                envs[i] = str.encode(env)
            print("[{}]env={}".format(time.strftime("%Y-%m-%d %H:%M:%S"),
                                      envs))

            time.sleep(20)
            xgb.rabit.init(envs)
            rank = xgb.rabit.get_rank()

            print("[{}]rank={} is going to run train".format(
                time.strftime("%Y-%m-%d %H:%M:%S"), rank))
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
        print("[%s]finally" % time.strftime("%Y-%m-%d %H:%M:%S"))
        if node == 'ps':
            if rabit_tracker is not None:
                rabit_tracker.join()
                print("[%s]ps joined" % time.strftime("%Y-%m-%d %H:%M:%S"))
        else:
            xgb.rabit.finalize()
            print("[%s]xgb.rabit.finalize() done" %
                  time.strftime("%Y-%m-%d %H:%M:%S"))
        print("[%s]---------------" % time.strftime("%Y-%m-%d %H:%M:%S"))
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
    # TODO(weiguoz) remove me
    print("[%s]mock train. sleep 30sec" % time.strftime("%Y-%m-%d %H:%M:%S"))
    time.sleep(30)
    return

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
