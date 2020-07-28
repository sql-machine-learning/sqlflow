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

try:
    import sqlflow_models  # noqa: F401
except:  # noqa: E722
    pass

from tensorflow.estimator import BoostedTreesClassifier  # noqa: F401
from tensorflow.estimator import BoostedTreesRegressor  # noqa: F401
from tensorflow.estimator import DNNClassifier  # noqa: F401
from tensorflow.estimator import DNNLinearCombinedClassifier  # noqa: F401
from tensorflow.estimator import DNNLinearCombinedRegressor  # noqa: F401
from tensorflow.estimator import DNNRegressor  # noqa: F401
from tensorflow.estimator import LinearClassifier  # noqa: F401
from tensorflow.estimator import LinearRegressor  # noqa: F401


def import_model_module(model, namespace):
    """
    Import the model module into namespace. For example,
    If model = "my_model_module.my_model", "my_model_module"
    would be imported into namespace.

    Args:
        model (str): the estimator name.
        namespace (dict): the namespace to be imported into.

    Returns:
        None.
    """
    # try import the custom model's python package, if the estimator is of
    # format: my_model_package.MyModel
    model_name_parts = model.split(".")
    if len(model_name_parts) == 2:
        module = model_name_parts[0]
        if module and module.lower() not in ['xgboost', 'sqlflow_models']:
            try:
                namespace[module] = __import__(module)
            except Exception as e:
                print("failed to import %s: %s" % (module, e))


def import_model(model):
    """
    Import TensorFlow estimator or Keras model from
    the given model name.

    Args:
        model (str): the model name.

    Returns:
        A imported TensorFlow estimator or Keras model.
    """
    import_model_module(model, globals())
    return eval(model)
