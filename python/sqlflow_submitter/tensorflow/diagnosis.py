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


class SQLFlowDiagnosis(Exception):
    pass


def check_and_load_estimator(estimator, model_params):
    try:
        return estimator(**model_params)
    except TypeError as e:
        raise SQLFlowDiagnosis("model attribute {0}".format(
            str(e).lstrip("__init__() ")))
