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
import copy
import inspect
import os
import re


class SQLFlowDiagnostic(Exception):
    pass


def check_and_load_estimator(estimator, model_params, warm_start_from=None):
    if warm_start_from is not None:
        estimator_func = estimator.__init__ if inspect.isclass(
            estimator) else estimator
        estimator_spec = inspect.getargspec(estimator_func)
        # The constructor of Estimator contains **kwargs or named parameter "warm_start_from"
        warm_start_from_key = "warm_start_from"
        if estimator_spec.keywords is not None or warm_start_from_key in estimator_spec.args:
            model_params = copy.copy(model_params)
            model_params[warm_start_from_key] = os.path.abspath(
                warm_start_from)
        else:
            raise NotImplementedError(
                "Incremental training is not supported in {}".format(
                    estimator))

    # load estimator class and diagnose the type error
    try:
        return estimator(**model_params)
    except TypeError as e:
        name = estimator.__name__
        # translate error message of TypeError to a SQLFLow user-friendly
        # diagnosis message
        re_missing_args = re.search(
            'missing (.*) required positional argument[s]*: (.*)', str(e))
        re_unexpected_args = re.search(
            'attribute got an unexpected keyword argument: (.*)', str(e))
        if re_missing_args:
            raise SQLFlowDiagnostic(
                "{0} missing {1} required attribute: {2}".format(
                    name, re_missing_args.group(1), re_missing_args.group(2)))
        elif re_unexpected_args:
            raise SQLFlowDiagnostic("%s get an unexpected attribute: %s", name,
                                    re_unexpected_args.group(1))
        else:
            raise SQLFlowDiagnostic("{0} attribute {1}".format(
                name,
                str(e).lstrip("__init__() ")))
