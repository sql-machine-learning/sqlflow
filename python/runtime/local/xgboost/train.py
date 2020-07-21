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
import types

import xgboost as xgb


def init_xgb_booster(load_pretrained_model, filename="my_model"):
    """
    Initialize XGBoost Booster from local saved model or just return None
    """
    if load_pretrained_model:
        bst = xgb.Booster()
        bst.load_model(filename)
        return bst
    return None


def train(train_dataset,
          train_params,
          model_params,
          val_dataset=None,
          load_pretrained_model=False):
    """ XGBoost local training API

    Args:
        train_dataset: Generator
            training dataset with XGBoost DMatrix generator
        train_params: dict
            training parameters, passed into `xgboost.train` API
        model_params: dict
            model parameters, the `model_param` arguments of `xgboost.train`
        val_dataset: Generator
            validation datasets generator with XGBoost DMatrix generator
        load_pretrained_model: bool
            load pre-trained model or not
    Returns:
        evaluation result
    """

    bst = init_xgb_booster(load_pretrained_model)
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

    return eval_result
