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

import tensorflow as tf
from tensorflow import feature_column as tfc


def numeric(field, dim=0):
    return tfc.numeric_column(field["feature_name"],
                              shape=[dim] if dim else field["shape"],
                              dtype=eval("tf." + field["dtype"]))


def embedding(field, dimension, combiner):
    if isinstance(field, dict):
        categorical = category_id(field, field["shape"][0], field["delimiter"])
    else:
        categorical = field
    ret = tfc.embedding_column(categorical, dimension, combiner)
    ret.key = categorical.key
    return ret


def indicator(field):
    if isinstance(field, dict):
        categorical = category_id(field, field["maxid"] + 1)
    else:
        categorical = field
    ret = tfc.indicator_column(categorical)
    ret.key = categorical.key
    return ret


def sparse(field, size, sep):
    field["is_sparse"], field["delimiter"], field["shape"] = True, sep, [size]
    return field


def category_id(field, size, sep=''):
    field = sparse(field, size, sep) if sep else field
    return tfc.categorical_column_with_identity(field["feature_name"], size)


def seq_category_id(field, size, sep=''):
    field = sparse(field, size, sep) if not field["delimiter"] else field
    ret = tfc.sequence_categorical_column_with_identity(
        field["feature_name"], size)
    ret.key = field["feature_name"]
    return ret


EVAL_GLOBALS = {'comma': ',', 'mean': 'mean', 'sum': 'sum', 'sqrtn': 'sqrtn'}
