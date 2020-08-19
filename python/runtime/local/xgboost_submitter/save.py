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

import xgboost as xgb
from sklearn2pmml import PMMLPipeline, sklearn2pmml

try:
    from xgboost.compat import XGBoostLabelEncoder
except:  # noqa: E722
    # xgboost==0.82.0 does not have XGBoostLabelEncoder
    # in xgboost.compat.py
    from xgboost.sklearn import XGBLabelEncoder as XGBoostLabelEncoder


def save_model_to_local_file(booster, model_params, file_name):
    """
    Save the XGBoost booster object to file. This method would
    serialize the XGBoost booster and save the PMML file.

    Args:
        booster: the XGBoost booster object.
        model_params (dict): the XGBoost model parameters.
        file_name (str): the file name to be save.

    Returns:
        None.
    """
    objective = model_params.get("objective")
    bst_meta = dict()

    if objective.startswith("binary:") or objective.startswith("multi:"):
        if objective.startswith("binary:"):
            num_class = 2
        else:
            num_class = model_params.get("num_class")
            assert num_class is not None and num_class > 0, \
                "num_class should not be None"

        # To fake a trained XGBClassifier, there must be "_le", "classes_",
        # inside XGBClassifier. See here:
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
    booster.save_model(file_name)
    booster.set_attr(scikit_learn=None)
    model.load_model(file_name)
    pipeline = PMMLPipeline([(model_type, model)])
    sklearn2pmml(pipeline, "{}.pmml".format(file_name))
