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

from datetime import datetime

import numpy as np
import xgboost as xgb
from runtime import db
from runtime.dbapi.paiio import PaiIOConnection
from runtime.model.metadata import load_metadata
from runtime.xgboost.dataset import DMATRIX_FILE_SEP, xgb_dataset

DEFAULT_PREDICT_BATCH_SIZE = 10000


def pred(datasource,
         select,
         feature_metas,
         feature_column_names,
         train_label_meta,
         pred_label_meta,
         result_table,
         is_pai=False,
         pai_table="",
         model_params=None,
         train_params=None,
         transform_fn=None,
         feature_column_code="",
         flags=None):
    rank = 0
    nworkers = len(flags.worker_hosts.split(",")) if flags else 1
    if nworkers > 1:
        if not is_pai:
            raise Exception(
                "XGBoost distributed predict is only supported on PAI")
        if flags.job_name != "worker":
            return  # ignore ps
        rank = flags.task_index
    pred_imp(datasource, select, feature_metas, feature_column_names,
             train_label_meta, pred_label_meta, result_table, is_pai,
             pai_table, model_params, train_params, transform_fn,
             feature_column_code, rank, nworkers)


def pred_imp(datasource,
             select,
             feature_metas,
             feature_column_names,
             train_label_meta,
             pred_label_meta,
             result_table,
             is_pai=False,
             pai_table="",
             model_params=None,
             train_params=None,
             transform_fn=None,
             feature_column_code="",
             rank=0,
             nworkers=1):
    print("rank={} nworkers={}".format(rank, nworkers))
    if not is_pai:
        conn = db.connect_with_data_source(datasource)
    else:
        conn = PaiIOConnection.from_table(pai_table)
    dpred = xgb_dataset(
        datasource=datasource,
        fn='predict.txt',
        dataset_sql=select,
        feature_metas=feature_metas,
        feature_column_names=feature_column_names,
        label_meta=None,
        is_pai=is_pai,
        pai_table=pai_table,
        pai_single_file=True,
        cache=True,
        batch_size=DEFAULT_PREDICT_BATCH_SIZE,
        rank=rank,
        nworkers=nworkers,
        transform_fn=transform_fn,
        feature_column_code=feature_column_code,
        raw_data_dir="predict.raw.dir")  # NOTE: default to use external memory
    bst = xgb.Booster({'nthread': 4})  # init model
    bst.load_model("my_model")  # load data
    print("{} Start predicting XGBoost model...".format(datetime.now()))
    if not model_params:
        model_params = load_metadata("model_meta.json")["attributes"]

    selected_cols = db.selected_cols(conn, select)

    feature_file_id = 0
    train_label_name = train_label_meta["feature_name"]
    pred_label_name = pred_label_meta["feature_name"]
    for pred_dmatrix in dpred:
        predict_and_store_result(bst, pred_dmatrix, feature_file_id,
                                 model_params, selected_cols, train_label_name,
                                 pred_label_name, feature_column_names,
                                 feature_metas, is_pai, conn, result_table,
                                 rank)
        feature_file_id += 1
    print("{} Done predicting. Predict table: {}".format(
        datetime.now(), result_table))


def predict_and_store_result(bst,
                             dpred,
                             feature_file_id,
                             model_params,
                             selected_cols,
                             train_label_name,
                             pred_label_name,
                             feature_column_names,
                             feature_metas,
                             is_pai,
                             conn,
                             result_table,
                             slice_id=0):
    preds = bst.predict(dpred)
    if model_params:
        obj = model_params["objective"]
        # binary:hinge output class labels
        if obj.startswith("binary:logistic"):
            preds = (preds > 0.5).astype(int)
        # multi:softmax output class labels
        elif obj.startswith("multi:softprob"):
            preds = np.argmax(np.array(preds), axis=1)
        # TODO(typhoonzero): deal with binary:logitraw when needed.
    else:
        # prediction output with multi-class job has two dimensions, this
        # is a temporary way, can remove this else branch when we can load
        # the model meta not only on PAI submitter.
        if len(preds.shape) == 2:
            preds = np.argmax(np.array(preds), axis=1)

    if is_pai:
        feature_file_read = open("predict.txt.raw", "r")
    else:
        feature_file_read = open(
            "predict.raw.dir/predict.txt_%d" % feature_file_id, "r")

    result_column_names = selected_cols[:]
    # remove train_label_name from result column, if train_label_name == "" or
    # the train_label_name is not selected, the index should be -1
    try:
        train_label_index = selected_cols.index(train_label_name)
    except ValueError:
        train_label_index = -1
    if train_label_index != -1:
        del result_column_names[train_label_index]
    result_column_names.append(pred_label_name)

    line_no = 0
    with db.buffered_db_writer(conn, result_table, result_column_names, 100,
                               slice_id) as w:
        while True:
            line = feature_file_read.readline()
            if not line:
                break
            # FIXME(typhoonzero): how to output columns that are not used
            # as features, like ids?
            row = [
                item
                for i, item in enumerate(line.strip().split(DMATRIX_FILE_SEP))
                if i != train_label_index
            ]
            row.append(preds[line_no])
            w.write(row)
            line_no += 1
