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
import re


class SQLFlowDiagnosis(Exception):
    pass


def check_and_load_estimator(estimator, model_params):
    # load estimator class and diagnose the type error
    try:
        name = estimator.__name__
        return estimator(**model_params)
    except TypeError as e:
        # translate error message of TypeError to a SQLFLow user-friendly
        # diagnosis message
        re_missing_args = re.search(
            'missing (.*) required positional argument[s]*: (.*)', str(e))
        re_unexpected_args = re.search(
            'attribute got an unexpected keyword argument: (.*)', str(e))
        if re_missing_args:
            raise SQLFlowDiagnosis(
                "{0} missing {1} required attribute: {2}".format(
                    name, re_missing_args.group(1), re_missing_args.group(2)))
        elif re_unexpected_args:
            raise SQLFlowDiagnosis("%s get an unexpected attribute: %s", name,
                                   re_unexpected_args.group(1))
        else:
            raise SQLFlowDiagnosis("{0} attribute {1}".format(
                name,
                str(e).lstrip("__init__() ")))
