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

import inspect
import json
import re

import six
import sqlflow_models
import tensorflow as tf
import xgboost


def parse_ctor_args(f, prefix=''):
    """Given an class or function, parse the class constructor/function details
    from its docstring

    For example, the docstring of sqlflow_models.DNNClassifier.__init__ is:
    '''DNNClassifier
    :param feature_columns: feature columns.
    :type feature_columns: list[tf.feature_column].
    :param hidden_units: number of hidden units.
    :type hidden_units: list[int].
    :param n_classes: List of hidden units per layer.
    :type n_classes: int.
    '''
    Calling parse_ctor_args(sqlflow_models.DNNClassifier, ":param") returns:
    {
        "feature_columns": "feature columns. :type feature_columns: list[tf.feature_column].",
        "hidden_units": "number of hidden units. :type hidden_units: list[int].",
        "n_classes": "List of hidden units per layer. :type n_classes: int."
    }
    And calling parse_ctor_args(parse_ctor_args) returns:
    {
        "f": "The class or function whose docstring to parse",
        "prefix": "The prefix of parameters in the docstring"
    }

    Args:
      f: The class or function whose docstring to parse
      prefix: The prefix of parameters in the docstring

    Returns:
      A dict with parameters as keys and splitted docstring as values
    """

    try:
        doc = f.__init__.__doc__
    except:
        doc = ''
    doc = doc if doc else f.__doc__
    if doc is None:
        doc = ''
    arg_list = list(inspect.signature(f).parameters)
    args = '|'.join(arg_list)
    arg_re = re.compile(r'(?<=\n)\s*%s\s*(%s)\s*:\s*' % (prefix, args),
                        re.MULTILINE)
    total = arg_re.split(six.ensure_str(doc))
    # Trim *args and **kwargs if any:
    total[-1] = re.sub(r'(?<=\n)\s*[\\*]+kwargs\s*:.*', '', total[-1], 1,
                       re.M | re.S)

    return dict(
        zip(total[1::2],
            [' '.join(doc.split()).replace("`", "'") for doc in total[2::2]]))


def print_param_doc(*modules):
    param_doc = {}  # { "class_names": {"parameters": "splitted docstrings"} }
    for module in modules:
        models = filter(lambda m: inspect.isclass(m[1]),
                        inspect.getmembers(__import__(module)))
        for name, cls in models:
            param_doc['{}.{}'.format(module,
                                     name)] = parse_ctor_args(cls, ':param')
    print(json.dumps(param_doc))


def print_tf_model_doc():
    tf_estimators = [
        "DNNClassifier",
        "DNNRegressor",
        "LinearClassifier",
        "LinearRegressor",
        "BoostedTreesClassifier",
        "BoostedTreesRegressor",
        "DNNLinearCombinedClassifier",
        "DNNLinearCombinedRegressor",
    ]

    param_doc = {}
    for cls in tf_estimators:
        param_doc[cls] = parse_ctor_args(getattr(tf.estimator, cls))

    print(json.dumps(param_doc))


def print_tf_optimizer_doc():
    # TensorFlow optimizers
    tf_optimizers = [
        "Adadelta",
        "Adagrad",
        "Adam",
        "Adamax",
        "Ftrl",
        "Nadam",
        "RMSprop",
        "SGD",
    ]

    param_doc = {}
    for cls in tf_optimizers:
        param_doc[cls] = parse_ctor_args(getattr(tf.optimizers, cls))

    print(json.dumps(param_doc))


def print_xgboost_model_doc():
    # xgboost models:  gbtree, gblinear or dart
    model_doc = parse_ctor_args(xgboost.XGBModel)
    if 'booster' in model_doc:
        del model_doc['booster']

    # FIXME(sneaxiy): types and docs of some parameters of XGBoost models
    # cannot be extracted from Python doc string automatically.
    extra_param_doc = {
        "max_bin":
        "Only used if tree_method is set to hist, Maximum number of discrete bins to bucket continuous features.",
    }

    extra_param_type = {
        "base_score": "float",
    }

    for param, doc in extra_param_doc.items():
        if param not in model_doc:
            model_doc[param] = doc

    for param, type in extra_param_type.items():
        if param not in model_doc:
            continue

        if not model_doc[param].lower().strip().startswith(type):
            model_doc[param] = type + " " + model_doc[param]

    all_docs = {}
    for model in ['xgboost.gbtree', 'xgboost.gblinear', 'xgboost.dart']:
        all_docs[model] = model_doc

    print(json.dumps(all_docs))
