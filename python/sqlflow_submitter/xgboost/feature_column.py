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
]


# TODO(sneaxiy): implement faster and proper hash algorithm
def hashing(x):
    return hash(x)  # use builtin hash function


def apply_transform_on_value(feature, transform_fn):
    if len(feature) == 1:  # Dense input is like (value, )
        return transform_fn(feature[0]),
    else:  # Sparse input is like (indices, values, dense_shape)
        return feature[0], transform_fn(feature[1]), feature[2]


class BaseColumnTransformer(object):
    def _set_field_names(self, field_names):
        self.field_names = field_names

    def get_column_names(self):
        raise NotImplementedError()

    def __call__(self, inputs):
        raise NotImplementedError()


class CategoricalColumnTransformer(BaseColumnTransformer):
    pass


class NumericColumnTransformer(BaseColumnTransformer):
    def __init__(self, key, shape):
        self.key = key
        self.shape = shape

    def _set_field_names(self, field_names):
        BaseColumnTransformer._set_field_names(self, field_names)
        self.column_idx = self.field_names.index(self.key)

    def __call__(self, inputs):
        return inputs[self.column_idx]

    def get_column_names(self):
        return [self.key]


def numeric_column(key, shape):
    return NumericColumnTransformer(key, shape)


class BucketizedColumnTransformer(CategoricalColumnTransformer):
    def __init__(self, source_column, boundaries):
        assert boundaries == sorted(
            boundaries), "Boundaries must be sorted in ascending order"
        self.source_column = source_column
        self.boundaries = boundaries

    def _set_field_names(self, field_names):
        CategoricalColumnTransformer._set_field_names(self, field_names)
        self.source_column._set_field_names(field_names)

    def get_column_names(self):
        return self.source_column.get_column_names()

    def __call__(self, inputs):
        return apply_transform_on_value(
            self.source_column(inputs),
            lambda x: np.searchsorted(self.boundaries, x))


def bucketized_column(source_column, boundaries):
    return BucketizedColumnTransformer(source_column, boundaries)


class CategoricalColumnWithIdentityTransformer(CategoricalColumnTransformer):
    def __init__(self, key, num_buckets, default_value=None):
        self.key = key
        self.num_buckets = num_buckets
        self.default_value = default_value

    def _set_field_names(self, field_names):
        CategoricalColumnTransformer._set_field_names(self, field_names)
        self.column_idx = self.field_names.index(self.key)

    def get_column_names(self):
        return [self.key]

    def __call__(self, inputs):
        def transform_fn(slot_value):
            invalid_index = slot_value < 0 or slot_value >= self.num_buckets
            if any(invalid_index):
                if self.default_value is not None:
                    slot_value[invalid_index] = self.default_value
                else:
                    raise ValueError(
                        'The categorical value of column {} out of range [0, {})'
                        .format(self.field_names[self.column_idx],
                                self.num_buckets))
            return slot_value

        return apply_transform_on_value(inputs[self.column_idx], transform_fn)


def categorical_column_with_identity(key, num_buckets, default_value=None):
    return CategoricalColumnWithIdentityTransformer(key, num_buckets,
                                                    default_value)


class CategoricalColumnWithVocabularyList(CategoricalColumnTransformer):
    def __init__(self, key, vocabulary_list):
        self.key = key
        self.vocabulary_list = vocabulary_list

    def _set_field_names(self, field_names):
        CategoricalColumnTransformer._set_field_names(self, field_names)
        self.column_idx = self.field_names.index(self.key)

    def get_column_names(self):
        return [self.key]

    def __call__(self, inputs):
        def transform_fn(slot_value):
            if isinstance(slot_value, np.ndarray):
                output = np.ndarray(slot_value.shape)
                for i in six.moves.range(slot_value.size):
                    output[i] = self.vocabulary_list.index(slot_value[i])
            else:
                output = self.vocabulary_list.index(slot_value)

            return output

        return apply_transform_on_value(inputs[self.column_idx], transform_fn)


def categorical_column_with_vocabulary_list(key, vocabulary_list):
    return CategoricalColumnWithVocabularyList(key, vocabulary_list)


class CategoricalColumnWithHashBucketTransformer(CategoricalColumnTransformer):
    def __init__(self, key, hash_bucket_size, dtype='string'):
        self.key = key
        self.hash_bucket_size = hash_bucket_size
        self.dtype = dtype

    def _set_field_names(self, field_names):
        CategoricalColumnTransformer._set_field_names(self, field_names)
        self.column_idx = self.field_names.index(self.key)

    def get_column_names(self):
        return [self.key]

    def __call__(self, inputs):
        def transform_fn(slot_value):
            if isinstance(slot_value, np.ndarray):
                output = np.ndarray(slot_value.shape)
                for i in six.moves.range(slot_value.size):
                    output[i] = hashing(slot_value[i])
            else:
                output = hashing(slot_value)

            output %= self.hash_bucket_size
            return output

        return apply_transform_on_value(inputs[self.column_idx], transform_fn)


def categorical_column_with_hash_bucket(key, hash_bucket_size, dtype='string'):
    return CategoricalColumnWithHashBucketTransformer(key, hash_bucket_size,
                                                      dtype)


class ComposedColumnTransformer(BaseColumnTransformer):
    def __init__(self, feature_column_names, *columns):
        for column in columns:
            assert isinstance(column, BaseColumnTransformer)

        assert len(columns) != 0, "No feature column found"

        self.columns = columns
        self._set_field_names(feature_column_names)

    def get_column_names(self):
        return ['/'.join(column.get_column_names()) for column in self.columns]

    def _set_field_names(self, field_names):
        BaseColumnTransformer._set_field_names(self, field_names)
        for column in self.columns:
            column._set_field_names(field_names)

    def __call__(self, inputs):
        return tuple([column(inputs) for column in self.columns])
