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


def get_import_name(estimator_name):
    # try import the custom model's python package, if the estimator is of format: my_model_package.MyModel
    model_name_parts = estimator_name.split(".")
    if len(model_name_parts) == 2:
        if model_name_parts[0].lower(
        ) != "xgboost" and model_name_parts[0].lower() != "sqlflow_models":
            return model_name_parts[0]
    return ""
