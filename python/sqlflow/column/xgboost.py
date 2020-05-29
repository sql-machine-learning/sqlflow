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

from sqlflow_submitter.xgboost import feature_column as xfc


def numeric(field):
    return xfc.numeric_column(field["feature_name"], shape=field["shape"])


def bucket(field, boundaries):
    if isinstance(field, dict):
        numerical = numeric(field)
    else:
        numerical = field
    ret = xfc.bucketized_column(numerical, boundaries)
    ret.key = numerical.key
    return ret


def indicator(field):
    if isinstance(field, dict):
        categorical = category_id(field, field["maxid"] + 1)
    else:
        categorical = field
    ret = xfc.indicator_column(categorical)
    ret.key = categorical.key
    return ret


def category_id(field, size):
    if field["vocab"]:
        return xfc.categorical_column_with_vocabulary_list(
            field["feature_name"], vocabulary_list=field["vocab"])
    else:
        return xfc.categorical_column_with_identity(field["feature_name"],
                                                    size)


def category_hash(field, size):
    return xfc.categorical_column_with_hash_bucket(field["feature_name"], size,
                                                   field["dtype"])


EVAL_GLOBALS = {}
