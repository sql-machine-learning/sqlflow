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
# limitations under the License
""" XGBoost Local Training.
This module launches a XGBoost training task on host.
"""
import os
import types

import runtime.db as db
import runtime.temp_file as temp_file
import runtime.xgboost as xgboost_extended
import xgboost as xgb
from runtime.feature.compile import compile_ir_feature_columns
from runtime.feature.derivation import (get_ordered_field_descs,
                                        infer_feature_columns)
from runtime.local.xgboost_submitter.save import save_model_to_local_file
from runtime.model import EstimatorType, Model, collect_metadata
from runtime.xgboost.dataset import xgb_dataset


def train(original_sql,
          model_image,
          estimator_string,
          datasource,
          select,
          validation_select,
          model_params,
          train_params,
          validation_params,
          feature_column_map,
          label_column,
          save,
          load=None):
    """
    Train, evaluate and save the XGBoost model locally.

    Args:
        original_sql (str): the original SQL statement.
        model_image (str): the model repo docker image.
        estimator (str): the XGBoost booster type like xgboost.gbtree.
        datasource (str): the database connection URI.
        select (str): the SQL statement for training.
        validation_select (str): the SQL statement for evaluation.
        model_params (dict): the XGBoost model parameters.
        train_params (dict): the training parameters, can have
                             disk_cache(bool), batch_size(int), epoch(int)
                             settings in the dict.
        validation_params (dict): the validation parameters. Not used
                                  currently.
        feature_column_map (dict): the feature column map to do derivation.
        label_column (FeatureColumn): the label column.
        save (str): the table name to save the trained model and meta.
        load (str): the table name to load the pretrained model.

    Returns:
        A dict which indicates the evaluation result.
    """
    conn = db.connect_with_data_source(datasource)
    fc_map_ir, fc_label_ir = infer_feature_columns(conn,
                                                   select,
                                                   feature_column_map,
                                                   label_column,
                                                   n=1000)
    fc_map = compile_ir_feature_columns(fc_map_ir, EstimatorType.XGBOOST)

    feature_column_list = fc_map["feature_columns"]
    field_descs = get_ordered_field_descs(fc_map_ir)
    feature_column_names = [fd.name for fd in field_descs]
    feature_metas = dict([(fd.name, fd.to_dict(dtype_to_string=True))
                          for fd in field_descs])
    label_meta = fc_label_ir.get_field_desc()[0].to_dict(dtype_to_string=True)

    # NOTE: in the current implementation, we are generating a transform_fn
    # from the COLUMN clause. The transform_fn is executed during the process
    # of dumping the original data into DMatrix SVM file.
    transform_fn = xgboost_extended.feature_column.ComposedColumnTransformer(
        feature_column_names, *feature_column_list)

    disk_cache = train_params.pop("disk_cache", False)
    batch_size = train_params.pop("batch_size", None)
    epoch = train_params.pop("epoch", 1)
    num_workers = train_params.pop("num_workers", 1)

    def build_dataset(fn, slct):
        return xgb_dataset(datasource,
                           fn,
                           slct,
                           feature_metas,
                           feature_column_names,
                           label_meta,
                           cache=disk_cache,
                           batch_size=batch_size,
                           epoch=epoch,
                           transform_fn=transform_fn)

    file_name = "my_model"
    if load:
        Model.load_from_db(datasource, load)
        bst = xgb.Booster()
        bst.load_model(file_name)
    else:
        bst = None

    with temp_file.TemporaryDirectory() as tmp_dir_name:
        train_fn = os.path.join(tmp_dir_name, 'train.txt')
        val_fn = os.path.join(tmp_dir_name, 'val.txt')
        train_dataset = build_dataset(train_fn, select)
        if validation_select:
            val_dataset = build_dataset(val_fn, validation_select)
        else:
            val_dataset = None

        eval_result = dict()
        watchlist = [None]
        if val_dataset:
            # The `xgboost.train` API only accepts the XGBoost DMatrix
            # object as the training or validation dataset, so we should
            # convert the generator to DMatrix.
            if isinstance(val_dataset, types.GeneratorType):
                val_dataset = list(val_dataset)[0]
            watchlist.append((val_dataset, "validate"))

        for per_batch_dmatrix in train_dataset:
            watchlist[0] = (per_batch_dmatrix, "train")
            bst = xgb.train(model_params,
                            per_batch_dmatrix,
                            evals=watchlist,
                            evals_result=eval_result,
                            xgb_model=bst,
                            **train_params)
            print("Evaluation result: %s" % eval_result)

    meta = collect_metadata(original_sql=original_sql,
                            select=select,
                            validation_select=validation_select,
                            model_repo_image=model_image,
                            class_name=estimator_string,
                            attributes=model_params,
                            features=fc_map_ir,
                            label=fc_label_ir,
                            evaluation=eval_result,
                            num_workers=num_workers)

    save_model_to_local_file(bst, model_params, file_name)
    model = Model(EstimatorType.XGBOOST, meta)
    model.save_to_db(datasource, save)
    conn.close()
    return eval_result
