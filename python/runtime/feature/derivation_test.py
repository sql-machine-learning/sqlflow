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

import json
import unittest

import runtime.feature.column as fc
import runtime.feature.derivation as fd
import runtime.testing as testing
from runtime.feature.column import (CategoryIDColumn, CrossColumn,
                                    EmbeddingColumn, NumericColumn)
from runtime.feature.field_desc import DataFormat, DataType, FieldDesc


class TestCSVRegex(unittest.TestCase):
    def test_csv_strings(self):
        csv_strs = [
            "1,2,3,4",
            "1.3,-3.2,132,32",
            "33,-33",
            "33,-33,",
            " 33 , -70 , 80 , ",
            " 33 , -70 , 80 ,",
            " 33 , -70 , 80, ",
            " 33 , -70 , 80,",
        ]

        for s in csv_strs:
            self.assertEqual(fd.infer_string_data_format(s), DataFormat.CSV)

    def test_non_csv_strings(self):
        non_csv_strs = [
            "100",
            "-100",
            "023,",
            ",123",
            "1.23",
        ]

        for s in non_csv_strs:
            self.assertEqual(fd.infer_string_data_format(s), DataFormat.PLAIN)


class TestKeyValueRegex(unittest.TestCase):
    def test_kv_strings(self):
        kv_strs = [
            "1:3 2:4\t 3:5  4:9",
            "0:1.3 10:-3.2 20:132 7:32",
            "3:33",
        ]

        for s in kv_strs:
            self.assertEqual(fd.infer_string_data_format(s), DataFormat.KV)

        general_kv_strs = [
            "1:3,3:4.0,8:3.0",
            "k:1.0,b:3.3,s:4.32",
            "unknown",  # will be parsed to {"unknown": 1.0}
        ]
        for s in general_kv_strs:
            self.assertEqual(fd.infer_string_data_format(s, ",", ":"),
                             DataFormat.KV)

        kv_str = "k::1.0|b::3.3|s::4.32"
        self.assertEqual(fd.infer_string_data_format(kv_str, "|", "::"),
                         DataFormat.KV)

    def test_non_kv_strings(self):
        non_kv_strs = [
            "100",
            "-10:100",
            "10.2:23,",
            "0:abc",
        ]

        for s in non_kv_strs:
            self.assertEqual(fd.infer_string_data_format(s), DataFormat.PLAIN)


class TestGetMaxIndexOfKeyValueString(unittest.TestCase):
    def infer_index(self, string):
        field_desc = FieldDesc(shape=[0])
        fd.fill_kv_field_desc(string, field_desc)
        return field_desc.shape[0]

    def test_infer_index(self):
        self.assertEqual(self.infer_index("1:2 3:4 2:1"), 4)
        self.assertEqual(self.infer_index("7:2\t 3:-4 10:20"), 11)
        self.assertEqual(self.infer_index("1:7"), 2)


@unittest.skipUnless(testing.get_driver() in ["mysql", "hive"],
                     "skip non MySQL and Hive tests")
class TestFeatureDerivationWithMockedFeatures(unittest.TestCase):
    def check_json_dump(self, features):
        dump_json = json.dumps(features, cls=fc.JSONEncoderWithFeatureColumn)
        new_features = json.loads(dump_json,
                                  cls=fc.JSONDecoderWithFeatureColumn)
        new_dump_json = json.dumps(new_features,
                                   cls=fc.JSONEncoderWithFeatureColumn)
        self.assertEqual(dump_json, new_dump_json)

    def test_without_cross(self):
        features = {
            'feature_columns': [
                EmbeddingColumn(dimension=256, combiner="mean", name="c3"),
                EmbeddingColumn(category_column=CategoryIDColumn(
                    FieldDesc(name="c5",
                              dtype=DataType.INT64,
                              shape=[10000],
                              delimiter=",",
                              is_sparse=True),
                    bucket_size=5000),
                                dimension=64,
                                combiner="sqrtn",
                                name="c5"),
            ]
        }

        label = NumericColumn(
            FieldDesc(name="class", dtype=DataType.INT64, shape=[1]))

        select = "select c1, c2, c3, c4, c5, c6, class " \
                 "from feature_derivation_case.train"
        conn = testing.get_singleton_db_connection()
        features, label = fd.infer_feature_columns(conn, select, features,
                                                   label)

        self.check_json_dump(features)
        self.check_json_dump(label)

        self.assertEqual(len(features), 1)
        self.assertTrue("feature_columns" in features)
        features = features["feature_columns"]
        self.assertEqual(len(features), 6)

        fc1 = features[0]
        self.assertTrue(isinstance(fc1, NumericColumn))
        self.assertEqual(len(fc1.get_field_desc()), 1)
        field_desc = fc1.get_field_desc()[0]
        self.assertEqual(field_desc.name, "c1")
        self.assertEqual(field_desc.dtype, DataType.FLOAT32)
        self.assertEqual(field_desc.format, DataFormat.PLAIN)
        self.assertFalse(field_desc.is_sparse)
        self.assertEqual(field_desc.shape, [1])

        fc2 = features[1]
        self.assertTrue(isinstance(fc2, NumericColumn))
        self.assertEqual(len(fc2.get_field_desc()), 1)
        field_desc = fc2.get_field_desc()[0]
        self.assertEqual(field_desc.name, "c2")
        self.assertEqual(field_desc.dtype, DataType.FLOAT32)
        self.assertEqual(field_desc.format, DataFormat.PLAIN)
        self.assertFalse(field_desc.is_sparse)
        self.assertEqual(field_desc.shape, [1])

        fc3 = features[2]
        self.assertTrue(isinstance(fc3, EmbeddingColumn))
        self.assertEqual(len(fc3.get_field_desc()), 1)
        field_desc = fc3.get_field_desc()[0]
        self.assertEqual(field_desc.name, "c3")
        self.assertEqual(field_desc.dtype, DataType.INT64)
        self.assertEqual(field_desc.format, DataFormat.CSV)
        self.assertFalse(field_desc.is_sparse)
        self.assertEqual(field_desc.shape, [4])
        self.assertEqual(fc3.dimension, 256)
        self.assertEqual(fc3.combiner, "mean")
        self.assertEqual(fc3.name, "c3")
        self.assertTrue(isinstance(fc3.category_column, CategoryIDColumn))
        self.assertEqual(fc3.category_column.bucket_size, 10)

        fc4 = features[3]
        self.assertTrue(isinstance(fc4, NumericColumn))
        self.assertEqual(len(fc4.get_field_desc()), 1)
        field_desc = fc4.get_field_desc()[0]
        self.assertEqual(field_desc.name, "c4")
        self.assertEqual(field_desc.dtype, DataType.FLOAT32)
        self.assertEqual(field_desc.format, DataFormat.CSV)
        self.assertFalse(field_desc.is_sparse)
        self.assertEqual(field_desc.shape, [4])

        fc5 = features[4]
        self.assertTrue(isinstance(fc5, EmbeddingColumn))
        self.assertEqual(len(fc5.get_field_desc()), 1)
        field_desc = fc5.get_field_desc()[0]
        self.assertEqual(field_desc.name, "c5")
        self.assertEqual(field_desc.dtype, DataType.INT64)
        self.assertEqual(field_desc.format, DataFormat.CSV)
        self.assertTrue(field_desc.is_sparse)
        self.assertEqual(field_desc.shape, [10000])
        self.assertEqual(fc5.dimension, 64)
        self.assertEqual(fc5.combiner, "sqrtn")
        self.assertEqual(fc5.name, "c5")
        self.assertTrue(isinstance(fc5.category_column, CategoryIDColumn))
        self.assertEqual(fc5.category_column.bucket_size, 5000)

        fc6 = features[5]
        self.assertTrue(isinstance(fc6, EmbeddingColumn))
        self.assertEqual(len(fc6.get_field_desc()), 1)
        field_desc = fc6.get_field_desc()[0]
        self.assertEqual(field_desc.name, "c6")
        self.assertEqual(field_desc.dtype, DataType.STRING)
        self.assertEqual(field_desc.format, DataFormat.PLAIN)
        self.assertFalse(field_desc.is_sparse)
        self.assertEqual(field_desc.shape, [1])
        self.assertEqual(field_desc.vocabulary, set(['FEMALE', 'MALE',
                                                     'NULL']))
        self.assertEqual(fc6.dimension, 128)
        self.assertEqual(fc6.combiner, "sum")
        self.assertEqual(fc6.name, "c6")
        self.assertTrue(isinstance(fc6.category_column, CategoryIDColumn))
        self.assertEqual(fc6.category_column.bucket_size, 3)

        self.assertTrue(isinstance(label, NumericColumn))
        self.assertEqual(len(label.get_field_desc()), 1)
        field_desc = label.get_field_desc()[0]
        self.assertEqual(field_desc.name, "class")
        self.assertEqual(field_desc.dtype, DataType.INT64)
        self.assertEqual(field_desc.format, DataFormat.PLAIN)
        self.assertFalse(field_desc.is_sparse)
        self.assertEqual(field_desc.shape, [])

    def test_with_cross(self):
        c1 = NumericColumn(
            FieldDesc(name='c1', dtype=DataType.INT64, shape=[1]))
        c2 = NumericColumn(
            FieldDesc(name='c2', dtype=DataType.INT64, shape=[1]))
        c4 = NumericColumn(
            FieldDesc(name='c4', dtype=DataType.INT64, shape=[1]))
        c5 = NumericColumn(
            FieldDesc(name='c5',
                      dtype=DataType.INT64,
                      shape=[1],
                      is_sparse=True))

        features = {
            'feature_columns': [
                c1,
                c2,
                CrossColumn([c4, c5], 128),
                CrossColumn([c1, c2], 256),
            ]
        }

        label = NumericColumn(
            FieldDesc(name='class', dtype=DataType.INT64, shape=[1]))
        select = "select c1, c2, c3, c4, c5, class " \
                 "from feature_derivation_case.train"

        conn = testing.get_singleton_db_connection()
        features, label = fd.infer_feature_columns(conn, select, features,
                                                   label)

        self.check_json_dump(features)
        self.check_json_dump(label)

        self.assertEqual(len(features), 1)
        self.assertTrue("feature_columns" in features)
        features = features["feature_columns"]
        self.assertEqual(len(features), 5)

        fc1 = features[0]
        self.assertTrue(isinstance(fc1, NumericColumn))
        self.assertEqual(len(fc1.get_field_desc()), 1)
        field_desc = fc1.get_field_desc()[0]
        self.assertEqual(field_desc.name, "c1")
        self.assertEqual(field_desc.dtype, DataType.FLOAT32)
        self.assertEqual(field_desc.format, DataFormat.PLAIN)
        self.assertFalse(field_desc.is_sparse)
        self.assertEqual(field_desc.shape, [1])

        fc2 = features[1]
        self.assertTrue(isinstance(fc2, NumericColumn))
        self.assertEqual(len(fc2.get_field_desc()), 1)
        field_desc = fc2.get_field_desc()[0]
        self.assertEqual(field_desc.name, "c2")
        self.assertEqual(field_desc.dtype, DataType.FLOAT32)
        self.assertEqual(field_desc.format, DataFormat.PLAIN)
        self.assertFalse(field_desc.is_sparse)
        self.assertEqual(field_desc.shape, [1])

        fc3 = features[2]
        self.assertTrue(isinstance(fc3, NumericColumn))
        self.assertEqual(len(fc3.get_field_desc()), 1)
        field_desc = fc3.get_field_desc()[0]
        self.assertEqual(field_desc.name, "c3")
        self.assertEqual(field_desc.dtype, DataType.INT64)
        self.assertEqual(field_desc.format, DataFormat.CSV)
        self.assertFalse(field_desc.is_sparse)
        self.assertEqual(field_desc.shape, [4])

        fc4 = features[3]
        self.assertTrue(isinstance(fc4, CrossColumn))
        self.assertEqual(len(fc4.get_field_desc()), 2)
        field_desc1 = fc4.get_field_desc()[0]
        self.assertEqual(field_desc1.name, "c4")
        self.assertEqual(field_desc1.dtype, DataType.FLOAT32)
        self.assertEqual(field_desc1.format, DataFormat.CSV)
        self.assertEqual(field_desc1.shape, [4])
        self.assertFalse(field_desc1.is_sparse)
        field_desc2 = fc4.get_field_desc()[1]
        self.assertEqual(field_desc2.name, "c5")
        self.assertEqual(field_desc2.dtype, DataType.INT64)
        self.assertEqual(field_desc2.format, DataFormat.CSV)
        self.assertTrue(field_desc2.is_sparse)

        fc5 = features[4]
        self.assertTrue(isinstance(fc5, CrossColumn))
        self.assertEqual(len(fc4.get_field_desc()), 2)
        field_desc1 = fc5.get_field_desc()[0]
        self.assertEqual(field_desc1.name, "c1")
        self.assertEqual(field_desc1.dtype, DataType.FLOAT32)
        self.assertEqual(field_desc1.format, DataFormat.PLAIN)
        self.assertEqual(field_desc1.shape, [1])
        self.assertFalse(field_desc1.is_sparse)
        field_desc2 = fc5.get_field_desc()[1]
        self.assertEqual(field_desc2.name, "c2")
        self.assertEqual(field_desc2.dtype, DataType.FLOAT32)
        self.assertEqual(field_desc2.format, DataFormat.PLAIN)
        self.assertEqual(field_desc2.shape, [1])
        self.assertFalse(field_desc2.is_sparse)

        self.assertTrue(isinstance(label, NumericColumn))
        self.assertEqual(len(label.get_field_desc()), 1)
        field_desc = label.get_field_desc()[0]
        self.assertEqual(field_desc.name, "class")
        self.assertEqual(field_desc.dtype, DataType.INT64)
        self.assertEqual(field_desc.format, DataFormat.PLAIN)
        self.assertFalse(field_desc.is_sparse)
        self.assertEqual(field_desc.shape, [])

    def test_no_column_clause(self):
        columns = [
            "sepal_length",
            "sepal_width",
            "petal_length",
            "petal_width",
        ]

        select = "select %s, class from iris.train" % ",".join(columns)

        conn = testing.get_singleton_db_connection()
        features = None
        label = NumericColumn(
            FieldDesc(name='class', dtype=DataType.INT64, shape=[1]))
        features, label = fd.infer_feature_columns(conn, select, features,
                                                   label)

        self.check_json_dump(features)
        self.check_json_dump(label)

        self.assertEqual(len(features), 1)
        self.assertTrue("feature_columns" in features)
        features = features["feature_columns"]
        self.assertEqual(len(features), 4)

        for i, f in enumerate(features):
            self.assertTrue(isinstance(f, NumericColumn))
            self.assertEqual(len(f.get_field_desc()), 1)
            field_desc = f.get_field_desc()[0]
            self.assertEqual(field_desc.name, columns[i])
            self.assertEqual(field_desc.dtype, DataType.FLOAT32)
            self.assertEqual(field_desc.format, DataFormat.PLAIN)
            self.assertFalse(field_desc.is_sparse)
            self.assertEqual(field_desc.shape, [1])

        self.assertTrue(isinstance(label, NumericColumn))
        self.assertEqual(len(label.get_field_desc()), 1)
        field_desc = label.get_field_desc()[0]
        self.assertEqual(field_desc.name, "class")
        self.assertEqual(field_desc.dtype, DataType.INT64)
        self.assertEqual(field_desc.format, DataFormat.PLAIN)
        self.assertFalse(field_desc.is_sparse)
        self.assertEqual(field_desc.shape, [])


if __name__ == '__main__':
    unittest.main()
