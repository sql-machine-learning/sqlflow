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
import types

import runtime.temp_file as temp_file
import xgboost as xgb
from runtime.feature.compile import compile_ir_feature_columns
from runtime.feature.derivation import get_ordered_field_descs
from runtime.model import EstimatorType, Model, collect_metadata, oss
from runtime.pai.pai_distributed import define_tf_flags
from runtime.step.xgboost.save import save_model_to_local_file
from runtime.xgboost.dataset import xgb_dataset
from runtime.xgboost.feature_column import ComposedColumnTransformer
# TODO(typhoonzero): mv dist_train here when refactor finishes
from runtime.xgboost.train import dist_train


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
          load=None,
          pai_table="",
          pai_val_table=""):
    is_pai = True if pai_table != "" else False
    is_dist_train = False
    FLAGS = None
    oss_model_dir = ""

    if is_pai:
        FLAGS = define_tf_flags()
        num_workers = len(FLAGS.worker_hosts.split(","))
        is_dist_train = num_workers > 1
        oss_model_dir = FLAGS.sqlflow_oss_modeldir
        try:
            oss_path_to_load = train_params.pop("oss_path_to_load")
            if load:
                oss.load_file(oss_path_to_load, "my_model")
        except:  # noqa: E722
            pass

    feature_columns = compile_ir_feature_columns(feature_column_map,
                                                 EstimatorType.XGBOOST)
    field_descs = get_ordered_field_descs(feature_column_map)
    feature_column_names = [fd.name for fd in field_descs]
    feature_metas = dict([(fd.name, fd.to_dict(dtype_to_string=True))
                          for fd in field_descs])
    label_meta = label_column.get_field_desc()[0].to_dict(dtype_to_string=True)

    transform_fn = ComposedColumnTransformer(
        feature_column_names, *feature_columns["feature_columns"])

    batch_size = train_params.pop("batch_size", None)
    epoch = train_params.pop("epoch", 1)
    load_pretrained_model = True if load else False
    disk_cache = train_params.pop("disk_cache", False)

    if is_dist_train:
        # NOTE(typhoonzero): dist_train returns None
        dist_train(flags=FLAGS,
                   datasource=datasource,
                   select=select,
                   model_params=model_params,
                   train_params=train_params,
                   feature_metas=feature_metas,
                   feature_column_names=feature_column_names,
                   label_meta=label_meta,
                   validation_select=validation_select,
                   disk_cache=disk_cache,
                   batch_size=batch_size,
                   epoch=epoch,
                   load_pretrained_model=load_pretrained_model,
                   is_pai=True,
                   pai_train_table=pai_table,
                   pai_validate_table=pai_val_table,
                   oss_model_dir=oss_model_dir,
                   transform_fn=transform_fn,
                   feature_column_code=feature_column_map,
                   model_repo_image=model_image,
                   original_sql=original_sql)
    else:
        return local_train(original_sql,
                           model_image,
                           estimator_string,
                           datasource,
                           select,
                           validation_select,
                           model_params,
                           train_params,
                           feature_metas,
                           feature_column_names,
                           feature_column_map,
                           label_column,
                           transform_fn,
                           save,
                           load=load,
                           is_pai=is_pai,
                           oss_model_dir=oss_model_dir)


def local_train(original_sql,
                model_image,
                estimator_string,
                datasource,
                select,
                validation_select,
                model_params,
                train_params,
                feature_metas,
                feature_column_names,
                feature_column_map,
                label_column,
                transform_fn,
                save,
                load="",
                is_pai=False,
                oss_model_dir=""):
    disk_cache = train_params.pop("disk_cache", False)
    batch_size = train_params.pop("batch_size", None)
    if batch_size is not None and batch_size < 0:
        batch_size = None

    epoch = train_params.pop("epoch", 1)
    num_workers = train_params.pop("num_workers", 1)
    label_meta_dict = label_column.get_field_desc()[0].to_dict(
        dtype_to_string=True)

    def build_dataset(fn, slct):
        return xgb_dataset(datasource,
                           fn,
                           slct,
                           feature_metas,
                           feature_column_names,
                           label_meta_dict,
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
                            features=feature_column_map,
                            label=label_column,
                            evaluation=eval_result,
                            num_workers=num_workers)

    save_model_to_local_file(bst, model_params, file_name)
    model = Model(EstimatorType.XGBOOST, meta)
    model.save_to_db(datasource, save)
    if is_pai and len(oss_model_dir) > 0:
        # TODO(typhoonzero): remove this since we are saving metas into db now.
        save_model(oss_model_dir, "my_model", model_params, train_params,
                   feature_metas, feature_column_names, label_column,
                   feature_column_map)

    return eval_result


def save_model(model_dir, filename, model_params, train_params, feature_metas,
               feature_column_names, label_meta, fc_map_ir):
    oss.save_file(model_dir, filename)
    oss.save_file(model_dir, "{}.pmml".format(filename))
    oss.save_metas(
        model_dir,
        1,
        "xgboost_model_desc",
        "",  # estimator = ""
        model_params,
        train_params,
        feature_metas,
        feature_column_names,
        label_meta,
        fc_map_ir)
