# Copyright 2019 The SQLFlow Authors. All rights reserved.
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
import logging
import os
import re

from launcher import register_data_source, config_helper, config_fields as cf, train, predict
from launcher.model_helper import load_launcher_model
from xgboost.automl_core import get_optimization_direction

from sqlflow_submitter.ant_xgboost.common import AntXGBoostError
from sqlflow_submitter.ant_xgboost.sqlflow_data_source import SQLFlowDSConfig, SQLFlowDataSource

register_data_source('sqlflow', SQLFlowDSConfig, SQLFlowDataSource)


def run_with_sqlflow(mode: str,
                     model_path: str,
                     learning_config: str,
                     data_source_config: str,
                     column_config: str,
                     valid_data_source_config: str = None):
    if mode not in (cf.JobType.TRAIN, cf.JobType.PREDICT):
        raise AntXGBoostError('Unknown run mode(%s) of ant-xgboost launcher.' % mode)
    is_train = mode == cf.JobType.TRAIN

    def parse_json_str(string: str):
        return json.loads(string.replace("\n", " ").replace("\t", " "))

    learning_fields = None
    if is_train:
        learning_config = parse_json_str(learning_config)
        # set non active convergence_criteria to record best_score while training
        if 'convergence_criteria' not in learning_config['params']:
            learning_config['params']['convergence_criteria'] = '-1:0:1.0'
        learning_fields = config_helper.load_config(cf.LearningFields, **learning_config)

    data_source_config = parse_json_str(data_source_config)
    ds_fields = cf.DataSourceFields('sqlflow', data_source_config)
    if valid_data_source_config:
        valid_data_source_config = parse_json_str(valid_data_source_config)
        val_ds_fields = cf.DataSourceFields('sqlflow', valid_data_source_config)
    else:
        val_ds_fields = None
    column_config = parse_json_str(column_config)
    col_fields = config_helper.load_config(cf.ColumnFields, **column_config)
    # hard code batch size of prediction with 1024
    data_builder = cf.DataBuilderFields() if is_train else cf.DataBuilderFields(batch_size=1024)
    data_fields = cf.DataFields(
        data_source=ds_fields,
        column_format=col_fields,
        builder=data_builder,
        valid_data_source=val_ds_fields)
    bst_path = os.path.join(model_path, 'sqlflow_booster')
    dump_fields = cf.DumpInfoFields(
        path=os.path.join(model_path, 'sqlflow_booster.txt'),
        with_stats=True,
        is_dump_fscore=True)
    model_fields = cf.ModelFields(model_path=bst_path, dump_conf=dump_fields)

    if is_train:
        try:
            # mkdir as tf.estimator
            if not os.path.exists(model_path):
                os.makedirs(model_path)
            train_fields = cf.TrainFields(learning_fields, data_fields, model_fields)
            # dump training config
            config = json.dumps(config_helper.dump_config(train_fields), indent=2)
            config = re.sub(r'"password": ".*"', '"password": "*****"', config)
            logging.warning("======xgboost training config======\n%s" % config)
            # run training pipeline
            train(train_fields)
            # record metrics separately
            model = load_launcher_model(model_fields)
            metrics = model.booster.attributes().copy()
            metrics['best_score'] = float(metrics['best_score'])
            metrics['best_iteration'] = int(metrics['best_iteration'])
            metrics['maximize_metric'] = get_optimization_direction(train_fields.xgboost_conf.params._asdict())
            with open(os.path.join(model_path, 'metrics.json'), 'a') as f:
                f.write(json.dumps(metrics))
        except Exception as e:
            raise AntXGBoostError('XGBoost training task failed: %s' % e)
    else:
        try:
            pred_fields = cf.PredictFields(data_fields, model_fields)
            predict(pred_fields)
        except Exception as e:
            raise AntXGBoostError('XGBoost prediction task failed: %s' % e)
