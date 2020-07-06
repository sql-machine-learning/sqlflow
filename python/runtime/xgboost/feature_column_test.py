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

import unittest

import numpy as np
import runtime.xgboost.feature_column as fc
from runtime.xgboost.feature_column import hashing


def get_hash(x, bucket_size):
    if isinstance(x, np.ndarray):
        ret = np.ndarray(x.shape, dtype='int64')
        for i in range(x.size):
            ret.put(i, hashing(x.take(i)) % bucket_size)
    else:
        ret = hashing(x) % bucket_size

    return ret


def build_one_hot_vector(idx, size):
    ret = np.zeros([size], dtype='int64')
    ret[idx] = 1
    return ret


class TestFeatureColumnBase(unittest.TestCase):
    def check(self, column, column_names, inputs, expected_outputs):
        if not isinstance(inputs, (list, tuple)):
            inputs = (inputs, )

        if not isinstance(expected_outputs, (list, tuple)):
            expected_outputs = (expected_outputs, )

        self.assertEqual(len(inputs), len(expected_outputs))

        column = fc.ComposedColumnTransformer(column_names, column)
        outputs = column((inputs, ))[0]

        self.assertEqual(len(inputs), len(outputs))

        if isinstance(outputs[0], np.ndarray):
            self.assertTrue(np.array_equal(outputs[0], expected_outputs[0]))
        else:
            self.assertEqual(outputs[0], expected_outputs[0])


class TestCategoricalAndIndicatorColumn(TestFeatureColumnBase):
    def test_vocabulary_list_category(self):
        column_names = ['x']

        column = fc.categorical_column_with_vocabulary_list(
            'x', vocabulary_list=['cat', 'dog', 'apple'])

        indicator_column = fc.indicator_column(column)

        self.check(column, column_names, 'cat', 0)
        self.check(indicator_column, column_names, 'cat',
                   np.array([1, 0, 0], dtype='int64'))

        self.check(column, column_names, 'dog', 1)
        self.check(indicator_column, column_names, 'dog',
                   np.array([0, 1, 0], dtype='int64'))

        self.check(column, column_names, 'apple', 2)
        self.check(indicator_column, column_names, 'apple',
                   np.array([0, 0, 1], dtype='int64'))

        input = np.array(
            [['apple', 'dog', 'dog', 'cat'], ['cat', 'apple', 'dog', 'cat']],
            dtype='str')
        category_output = np.array([[2, 1, 1, 0], [0, 2, 1, 0]], dtype='int64')
        self.check(column, column_names, input, category_output)

        indicator_output = np.array(
            [[[0, 0, 1], [0, 1, 0], [0, 1, 0], [1, 0, 0]],
             [[1, 0, 0], [0, 0, 1], [0, 1, 0], [1, 0, 0]]],
            dtype='int64')
        self.check(indicator_column, column_names, input, indicator_output)

    def test_identity_category(self):
        column_names = ['x']
        num_bucket = 3
        column = fc.categorical_column_with_identity('x',
                                                     num_bucket,
                                                     default_value=None)
        self.check(column, column_names, 0, 0)
        self.check(column, column_names, 1, 1)
        self.check(column, column_names, 2, 2)
        with self.assertRaises(ValueError):
            self.check(column, column_names, 3, 3)

        input = np.array([[0, 1, 2], [2, 1, 1]], dtype='int64')
        self.check(column, column_names, input, input)

        column = fc.categorical_column_with_identity('x',
                                                     num_bucket,
                                                     default_value=1)
        self.check(column, column_names, 3, 1)
        input = np.array([[0, 3, 2], [3, 1, 3]], dtype='int64')
        output = np.array([[0, 1, 2], [1, 1, 1]], dtype='int64')
        self.check(column, column_names, input, output)

        indicator_column = fc.indicator_column(column)
        self.check(
            indicator_column, column_names, input,
            np.array([[[1, 0, 0], [0, 1, 0], [0, 0, 1]],
                      [[0, 1, 0], [0, 1, 0], [0, 1, 0]]]))

    def test_hash_category(self):
        column_names = ['x']
        num_bucket = 100
        column = fc.categorical_column_with_hash_bucket('x', num_bucket)

        self.check(column, column_names, 'cat', get_hash('cat', num_bucket))
        self.check(column, column_names, 'dog', get_hash('dog', num_bucket))

        input = np.array([['a', 'b'], ['c', 'd']], dtype='str')
        output = get_hash(input, num_bucket)
        self.check(column, column_names, input, output)

        indicator_column = fc.indicator_column(column)
        output = np.array(
            [[
                build_one_hot_vector(get_hash('a', num_bucket), num_bucket),
                build_one_hot_vector(get_hash('b', num_bucket), num_bucket)
            ],
             [
                 build_one_hot_vector(get_hash('c', num_bucket), num_bucket),
                 build_one_hot_vector(get_hash('d', num_bucket), num_bucket)
             ]],
            dtype='int64')
        self.check(indicator_column, column_names, input, output)

    def test_bucketized_column(self):
        column_names = ['x']
        boundaries = [-1, 2, 3, 4]
        num_bucket = len(boundaries) + 1
        column = fc.bucketized_column(fc.numeric_column('x'),
                                      boundaries=boundaries)
        self.check(column, column_names, -2, 0)
        self.check(column, column_names, -1, 1)
        self.check(column, column_names, -0.5, 1)
        self.check(column, column_names, 2, 2)
        self.check(column, column_names, 2.5, 2)
        self.check(column, column_names, 3, 3)
        self.check(column, column_names, 3.5, 3)
        self.check(column, column_names, 4, 4)
        self.check(column, column_names, 5, 4)

        indicator_column = fc.indicator_column(column)
        input = np.array([[-2, -1, -0.5, 2], [2.5, 3, 3.5, 4]],
                         dtype='float32')
        output = [
            [
                build_one_hot_vector(0, num_bucket),
                build_one_hot_vector(1, num_bucket),
                build_one_hot_vector(1, num_bucket),
                build_one_hot_vector(2, num_bucket),
            ],
            [
                build_one_hot_vector(2, num_bucket),
                build_one_hot_vector(3, num_bucket),
                build_one_hot_vector(3, num_bucket),
                build_one_hot_vector(4, num_bucket),
            ],
        ]
        output = np.array(output, dtype='int64')
        self.check(indicator_column, column_names, input, output)


if __name__ == '__main__':
    unittest.main()
