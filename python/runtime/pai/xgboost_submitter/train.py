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

from runtime import db
from runtime.feature.compile import compile_ir_feature_columns
from runtime.feature.derivation import (get_ordered_field_descs,
                                        infer_feature_columns)
from runtime.model import EstimatorType, oss
from runtime.pai.pai_distributed import define_tf_flags
from runtime.xgboost.feature_column import ComposedColumnTransformer
from runtime.xgboost.train import dist_train
from runtime.xgboost.train import train as local_train


def train(datasource,
          estimator_string,
          select,
          validation_select,
          feature_columns,
          feature_column_names,
          feature_metas={},
          label_meta={},
          model_params={},
          train_params={},
          validation_metrics=["Accuracy"],
          disk_cache=False,
          save="",
          batch_size=None,
          epoch=1,
          validation_steps=1,
          verbose=0,
          max_steps=None,
          validation_start_delay_secs=0,
          validation_throttle_secs=0,
          save_checkpoints_steps=100,
          log_every_n_iter=10,
          load_pretrained_model=False,
          is_pai=True,
          pai_table="",
          pai_val_table="",
          feature_columns_code="",
          model_repo_image="",
          original_sql="",
          oss_model_dir_to_load="",
          feature_column_names_map=None):

    FLAGS = define_tf_flags()
    num_workers = len(FLAGS.worker_hosts.split(","))
    is_dist_train = num_workers > 1
    oss_model_dir = FLAGS.sqlflow_oss_modeldir

    if load_pretrained_model:
        oss.load_file(oss_model_dir_to_load, "my_model")

    # NOTE: in the current implementation, we are generating a transform_fn
    # from COLUMN clause. The transform_fn is executed during the process of
    # dumping the original data into DMatrix SVM file.
    transform_fn = ComposedColumnTransformer(feature_column_names,
                                             *feature_columns)

    if is_dist_train:
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
                   feature_column_code=feature_columns_code,
                   model_repo_image=model_repo_image,
                   original_sql=original_sql)
    else:
        local_train(datasource=datasource,
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
                    rank=0,
                    nworkers=1,
                    oss_model_dir=oss_model_dir,
                    transform_fn=transform_fn,
                    feature_column_code=feature_columns_code,
                    model_repo_image=model_repo_image,
                    original_sql=original_sql)


def train_step(original_sql,
               model_image,
               estimator_string,
               datasource,
               select,
               validation_select,
               pai_table,
               pai_val_table,
               model_params,
               train_params,
               feature_column_map,
               label_column,
               save,
               load=None):
    FLAGS = define_tf_flags()
    num_workers = len(FLAGS.worker_hosts.split(","))
    is_dist_train = num_workers > 1
    oss_model_dir = FLAGS.sqlflow_oss_modeldir

    oss_path_to_load = train_params.pop("oss_path_to_load")
    if load:
        oss.load_file(oss_path_to_load, "my_model")

    conn = db.connect_with_data_source(datasource)
    fc_map_ir, fc_label_ir = infer_feature_columns(conn,
                                                   select,
                                                   feature_column_map,
                                                   label_column,
                                                   n=1000)
    feature_columns = compile_ir_feature_columns(fc_map_ir,
                                                 EstimatorType.XGBOOST)
    field_descs = get_ordered_field_descs(fc_map_ir)
    feature_column_names = [fd.name for fd in field_descs]
    feature_metas = dict([(fd.name, fd.to_dict()) for fd in field_descs])
    label_meta = label_column.get_field_desc()[0].to_dict()

    transform_fn = ComposedColumnTransformer(
        feature_column_names, *feature_columns["feature_columns"])

    batch_size = train_params.pop("batch_size", None)
    epoch = train_params.pop("epoch", 1)
    load_pretrained_model = True if load else False
    disk_cache = train_params.pop("disk_cache", False)

    if is_dist_train:
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
                   feature_column_code=fc_map_ir,
                   model_repo_image=model_image,
                   original_sql=original_sql)
    else:
        local_train(datasource=datasource,
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
                    rank=0,
                    nworkers=1,
                    oss_model_dir=oss_model_dir,
                    transform_fn=transform_fn,
                    feature_column_code=fc_map_ir,
                    model_repo_image=model_image,
                    original_sql=original_sql)
