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


def explain(datasource, select, explainer, model_params, result_table, model):
    """
    Do explanation to a trained TensorFlow model.

    Args:
        datasource (str): the database connection string.
        select (str): the input data to predict.
        explainer (str): the explainer to explain the model.
        model_params (dict): the parameters for evaluation.
        result_table (str): the output data table.
        model (Model|str): the model object or where to load the model.

    Returns:
        None.
    """
    # TODO(sneaxiy): implement this method.
    pass
