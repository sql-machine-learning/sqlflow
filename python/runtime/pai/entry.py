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
import pickle
import types
from inspect import getargspec

from runtime import oss
from runtime.diagnostics import SQLFlowDiagnostic
from runtime.pai import explain, predict
from runtime.pai import train as tf_train
from runtime.pai.pai_distributed import define_tf_flags, set_oss_environs
from runtime.pai.xgboost import train as xgb_train
from runtime.tensorflow import is_tf_estimator


def call_fun(func, params):
    """Call a function with given params, entries in params will be treated
    as func' param if the key matches some argument name. Do not support 
    var-args in func.

    Arags:
        func: callable
            a Python callable object
        params: dict
            dict of params
    Returns:
        the return value of func if success

    Raises:
        SQLFlowDiagnostic if none-optional argument is not found in params
    """
    # getargspec returns (pos_args, var_args, dict_args, defaults)
    sig = getargspec(func)
    required_len = len(sig[0]) - (0 if sig[3] is None else len(sig[3]))
    # if func has dict args, pass all params into it
    if sig[2] is not None:
        return func(**params)

    # if func has no dict args, we need to remove non-param entries in params
    dict_args = dict()
    for i, name in enumerate(sig[0]):
        if i < required_len:
            if name not in params:
                raise SQLFlowDiagnostic("Non-default param is not passed:%s" %
                                        name)
        if name in params:
            dict_args[name] = params[name]
    return func(**dict_args)


def entrypoint():
    with open("train_params.pkl", "rb") as file:
        params = pickle.load(file)
    if params["entry_type"] == "train_tf":
        call_fun(tf_train.train, params)
    elif params["entry_type"] == "train_xgb":
        call_fun(xgb_train.train, params)
    elif params["entry_type"] == "predict_tf":
        call_fun(predict.predict_tf, params)
    elif params["entry_type"] == "explain_tf":
        call_fun(explain.explain_tf, params)


if __name__ == "__main__":
    FLAGS = define_tf_flags()
    set_oss_environs(FLAGS)
    entrypoint()
