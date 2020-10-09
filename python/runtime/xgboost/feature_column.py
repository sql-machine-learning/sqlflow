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

import hashlib

import numpy as np
import six

__all__ = [
    'numeric_column',
    'bucketized_column',
    'categorical_column_with_identity',
    'categorical_column_with_vocabulary_list',
    'categorical_column_with_hash_bucket',
    'indicator_column',
]

# TODO(sneaxiy): implement faster and proper hash algorithm
# We cannot use Python builtin hash here, because it would
# generate random results. See
# https://stackoverflow.com/questions/27522626/hash-function-in-python-3-3-returns-different-results-between-sessions
if six.PY2:

    def hashing(x):
        return long(hashlib.sha1(x).hexdigest(), 16)  # noqa: F821
else:

    def hashing(x):
        return int(hashlib.sha1(x.encode('utf-8')).hexdigest(), 16)


def elementwise_transform(array, transform_fn):
    vfunc = np.vectorize(transform_fn)
    return vfunc(array)


def apply_transform_on_value(feature, transform_fn):
    if len(feature) == 1:  # Dense input is like (value, )
        return transform_fn(feature[0]),
    else:  # Sparse input is like (indices, values, dense_shape)
        return feature[0], transform_fn(feature[1]), feature[2]


class BaseColumnTransformer(object):
    def _set_feature_column_names(self, names):
        self.names = names

    def get_feature_column_names(self):
        raise NotImplementedError()

    def __call__(self, inputs):
        raise NotImplementedError()


class CategoricalColumnTransformer(BaseColumnTransformer):
    def num_classes(self):
        raise NotImplementedError()


class NumericColumnTransformer(BaseColumnTransformer):
    def __init__(self, key, shape=(1, ), dtype='float32'):
        self.key = key
        self.shape = shape
        self.dtype = dtype

    def _set_feature_column_names(self, names):
        BaseColumnTransformer._set_feature_column_names(self, names)
        self.column_idx = self.names.index(self.key)

    def __call__(self, inputs):
        return inputs[self.column_idx]

    def get_feature_column_names(self):
        return [self.key]


def numeric_column(key, shape=(1, ), dtype='float32'):
    return NumericColumnTransformer(key, shape, dtype)


class BucketizedColumnTransformer(CategoricalColumnTransformer):
    def __init__(self, source_column, boundaries):
        for i in six.moves.range(len(boundaries) - 1):
            assert boundaries[i] < boundaries[i+1], \
                "Boundaries must be sorted in ascending order"
        self.source_column = source_column
        self.boundaries = boundaries

    def _set_feature_column_names(self, names):
        CategoricalColumnTransformer._set_feature_column_names(self, names)
        self.source_column._set_feature_column_names(names)

    def get_feature_column_names(self):
        return self.source_column.get_feature_column_names()

    def num_classes(self):
        return len(self.boundaries) + 1

    def __call__(self, inputs):
        return apply_transform_on_value(
            self.source_column(inputs),
            lambda x: np.searchsorted(self.boundaries, x, side='right'))


def bucketized_column(source_column, boundaries):
    return BucketizedColumnTransformer(source_column, boundaries)


class CategoricalColumnWithIdentityTransformer(CategoricalColumnTransformer):
    def __init__(self, key, num_buckets, default_value=None):
        self.key = key
        self.num_buckets = num_buckets
        self.default_value = default_value

    def _set_feature_column_names(self, names):
        CategoricalColumnTransformer._set_feature_column_names(self, names)
        self.column_idx = self.names.index(self.key)

    def get_feature_column_names(self):
        return [self.key]

    def num_classes(self):
        return self.num_buckets

    def __call__(self, inputs):
        def transform_fn(slot_value):
            def elementwise_transform_fn(x):
                if x >= 0 and x < self.num_buckets:
                    return x

                if self.default_value is not None:
                    return self.default_value
                else:
                    raise ValueError('The categorical value of column {} '
                                     'out of range [0, {})'.format(
                                         self.key, self.num_buckets))

            if isinstance(slot_value, np.ndarray):
                output = elementwise_transform(
                    slot_value, elementwise_transform_fn).astype(np.int64)
            else:
                output = elementwise_transform_fn(slot_value)
            return output

        return apply_transform_on_value(inputs[self.column_idx], transform_fn)


def categorical_column_with_identity(key, num_buckets, default_value=None):
    return CategoricalColumnWithIdentityTransformer(key, num_buckets,
                                                    default_value)


class CategoricalColumnWithVocabularyList(CategoricalColumnTransformer):
    def __init__(self, key, vocabulary_list):
        self.key = key
        self.vocabulary_list = vocabulary_list

    def _set_feature_column_names(self, names):
        CategoricalColumnTransformer._set_feature_column_names(self, names)
        self.column_idx = self.names.index(self.key)

    def get_feature_column_names(self):
        return [self.key]

    def num_classes(self):
        return len(self.vocabulary_list)

    def __call__(self, inputs):
        fn = lambda x: self.vocabulary_list.index(x)  # noqa: E731

        def transform_fn(slot_value):
            if isinstance(slot_value, np.ndarray):
                output = elementwise_transform(slot_value, fn).astype(np.int64)
            else:
                output = fn(slot_value)

            return output

        return apply_transform_on_value(inputs[self.column_idx], transform_fn)


def categorical_column_with_vocabulary_list(key, vocabulary_list):
    return CategoricalColumnWithVocabularyList(key, vocabulary_list)


class CategoricalColumnWithHashBucketTransformer(CategoricalColumnTransformer):
    def __init__(self, key, hash_bucket_size, dtype='string'):
        self.key = key
        self.hash_bucket_size = hash_bucket_size
        self.dtype = dtype

    def _set_feature_column_names(self, names):
        CategoricalColumnTransformer._set_feature_column_names(self, names)
        self.column_idx = self.names.index(self.key)

    def get_feature_column_names(self):
        return [self.key]

    def num_classes(self):
        return self.hash_bucket_size

    def __call__(self, inputs):
        fn = lambda x: hashing(x) % self.hash_bucket_size  # noqa: E731

        def transform_fn(slot_value):
            if isinstance(slot_value, np.ndarray):
                output = elementwise_transform(slot_value, fn).astype(np.int64)
                output = output.astype(np.int64)
            else:
                output = fn(slot_value)

            return output

        return apply_transform_on_value(inputs[self.column_idx], transform_fn)


def categorical_column_with_hash_bucket(key, hash_bucket_size, dtype='string'):
    return CategoricalColumnWithHashBucketTransformer(key, hash_bucket_size,
                                                      dtype)


class IndicatorColumnTransformer(BaseColumnTransformer):
    def __init__(self, categorical_column):
        assert isinstance(categorical_column, CategoricalColumnTransformer), \
            "categorical_column must be type of " \
            "CategoricalColumnTransformer but got {}".format(
                type(categorical_column))
        self.categorical_column = categorical_column

    def _set_feature_column_names(self, names):
        BaseColumnTransformer._set_feature_column_names(self, names)
        self.categorical_column._set_feature_column_names(names)

    def get_feature_column_names(self):
        return self.categorical_column.get_feature_column_names()

    def __call__(self, inputs):
        slot = self.categorical_column(inputs)
        assert len(
            slot
        ) == 1, "indicator_column does not accept sparse categorical feature"

        def transform_fn(slot_value):
            num_classes = self.categorical_column.num_classes()
            if isinstance(slot_value, np.ndarray):
                output = np.zeros([slot_value.size, num_classes],
                                  dtype=np.int64)
                for i in six.moves.range(slot_value.size):
                    output[i][slot_value.take(i)] = 1
                output = output.reshape(slot_value.shape + (num_classes, ))
            else:
                output = np.zeros((num_classes, ), dtype=np.int64)
                output[slot_value] = 1

            return output

        return apply_transform_on_value(slot, transform_fn)


def indicator_column(categorical_column):
    return IndicatorColumnTransformer(categorical_column)


class ComposedColumnTransformer(BaseColumnTransformer):
    def __init__(self, feature_column_names, *columns):
        for column in columns:
            assert isinstance(column, BaseColumnTransformer)

        assert len(columns) != 0, "No feature column found"

        self.columns = columns
        self._set_feature_column_names(feature_column_names)

    def get_feature_column_names(self):
        return [
            '/'.join(column.get_feature_column_names())
            for column in self.columns
        ]

    def _set_feature_column_names(self, names):
        BaseColumnTransformer._set_feature_column_names(self, names)
        for column in self.columns:
            column._set_feature_column_names(names)

    def __call__(self, inputs):
        return tuple([column(inputs) for column in self.columns])
