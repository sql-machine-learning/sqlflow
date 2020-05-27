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

import numpy as np

__all__ = [
    'numeric_column',
    'bucketized_column',
    'categorical_column_with_identity',
    'categorical_column_with_vocabulary_list',
    'categorical_column_with_hash_bucket',
    'sequence_categorical_column_with_identity',
    'crossed_column',
    'indicator_column',
]


# TODO(sneaxiy): implement faster and proper hashing algorithm
def hashing(x, bucket_size=None):
    h = hash(x)  # use builtin hashing function
    if bucket_size:
        h = abs(h) % bucket_size  # round to bucket_size
    return h


def safe_index(list, item):
    idx = list.index(item)
    assert idx >= 0 and idx < len(
        list), "cannot find item in list {} {}".format(idx, len(list))
    return idx


class BaseColumnTransformer(object):
    def set_field_names(self, field_names):
        self.field_names = field_names

    def __call__(self, inputs):
        raise NotImplementedError()


class CategoricalColumnTransformer(BaseColumnTransformer):
    def __call__(self, inputs):
        raise NotImplementedError()


class NumericColumnTransformer(BaseColumnTransformer):
    def __init__(self, key, shape, dtype):
        self.key = key
        self.shape = shape
        self.dtype = dtype

    def set_field_names(self, field_names):
        BaseColumnTransformer.set_field_names(self, field_names)
        self.column_idx = safe_index(self.field_names, self.key)

    def __call__(self, inputs):
        return inputs[self.column_idx]


def numeric_column(key, shape, dtype):
    return NumericColumnTransformer(key, shape, dtype)


class BucketizedColumnTransformer(CategoricalColumnTransformer):
    def __init__(self, source_column, boundaries):
        assert boundaries == sorted(
            boundaries), "Boundaries must be sorted in ascending order"
        self.source_column = source_column
        self.boundaries = boundaries

    def set_field_names(self, field_names):
        CategoricalColumnTransformer.set_field_names(self, field_names)
        self.source_column.set_field_names(field_names)

    def __call__(self, inputs):
        slot_value = self.source_column(inputs)
        return np.searchsorted(self.boundaries, slot_value)


def bucketized_column(source_column, boundaries):
    return BucketizedColumnTransformer(source_column, boundaries)


class CategoricalColumnWithIdentityTransformer(CategoricalColumnTransformer):
    def __init__(self, key, num_buckets, default_value=None):
        self.key = key
        self.num_buckets = num_buckets
        self.default_value = default_value

    def set_field_names(self, field_names):
        BaseColumnTransformer.set_field_names(self, field_names)
        self.column_idx = safe_index(self.field_names, self.key)

    def __call__(self, inputs):
        slot_value = inputs[self.column_idx]
        invalid_index = slot_value < 0 or slot_value >= self.num_buckets
        if any(invalid_index):
            if self.default_value is not None:
                slot_value[invalid_index] = self.default_value
            else:
                raise ValueError(
                    'The categorical value of column {} out of range [0, {})'.
                    format(self.field_names[self.column_idx],
                           self.num_buckets))
        else:
            return slot_value


def categorical_column_with_identity(key, num_buckets, default_value=None):
    return CategoricalColumnWithIdentityTransformer(key, num_buckets,
                                                    default_value)


class CategoricalColumnWithHashBucketTransformer(CategoricalColumnTransformer):
    def __init__(self, key, hash_bucket_size, dtype='string'):
        self.key = key
        self.hash_bucket_size = hash_bucket_size
        self.dtype = dtype

    def set_field_names(self, field_names):
        BaseColumnTransformer.set_field_names(self, field_names)
        self.column_idx = safe_index(self.field_names, self.key)

    def __call__(self, inputs):
        slot_value = inputs[self.column_idx]
        output = np.ndarray(slot_value.shape)
        for i in six.moves.range(slot_value.size):
            output[i] = hashing(slot_value[i], hash_bucket_size)


def categorical_column_with_hash_bucket(key, hash_bucket_size, dtype='string'):
    return CategoricalColumnWithHashBucketTransformer(key, hash_bucket_size,
                                                      dtype)


class SequenceCategoricalColumnWithIdentityTransformer(
        CategoricalColumnTransformer):
    def __init__(self, key, num_buckets, default_value=None):
        self.key = key
        self.num_buckets = num_buckets
        self.default_value = default_value

    def set_field_names(self, field_names):
        BaseColumnTransformer.set_field_names(self, field_names)
        self.column_idx = safe_index(self.field_names, self.key)

    def __call__(self, inputs):
        raise ValueError('Not supported yet')


def sequence_categorical_column_with_identity(key,
                                              num_buckets,
                                              default_value=None):
    return SequenceCategoricalColumnWithIdentityTransformer(
        key, num_buckets, default_value)


class CrossedColumnTransformer(BaseColumnTransformer):
    def __init__(self, keys, hash_bucket_size, hash_key=None):
        self.keys = keys
        self.hash_bucket_size = hash_bucket_size
        self.hash_key = hash_key

    def set_field_names(self, field_names):
        BaseColumnTransformer.set_field_names(self, field_names)
        self.columns = [safe_index(self.field_names, key) for key in self.keys]

    def __call__(self, inputs):
        raise ValueError('Not supported yet')


def cross_column(keys, hash_bucket_size, hash_key=None):
    return CrossedColumnTransformer(keys, hash_bucket_size, hash_key)


class IndicatorColumnTransformer(BaseColumnTransformer):
    def __init__(self, categorical_column):
        assert isinstance(categorical_column, CategoricalColumnTransformer)
        self.categorical_column = categorical_column

    def set_field_names(self, field_names):
        BaseColumnTransformer.set_field_names(self, field_names)
        self.categorical_column.set_field_names(field_names)

    def __call__(self, inputs):
        raise ValueError('Not supported yet')


def indicator_column(categorical_column):
    return IndicatorColumnTransformer(categorical_column)


class ComposedColumnTransformer(BaseColumnTransformer):
    def __init__(self, *columns):
        for column in columns:
            assert isinstance(column, BaseColumnTransformer)

        self.columns = columns

    def set_field_names(self, field_names):
        BaseColumnTransformer.set_field_names(self, field_names)
        for column in self.columns:
            column.set_field_names(field_names)

    def __call__(self, inputs):
        return [column(inputs) for column in self.columns]
