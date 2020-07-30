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

from runtime.feature.column import (BucketColumn, CategoryHashColumn,
                                    CategoryIDColumn, CrossColumn,
                                    EmbeddingColumn, IndicatorColumn,
                                    NumericColumn, SeqCategoryIDColumn)
from runtime.feature.compile import compile_ir_feature_columns
from runtime.feature.field_desc import DataType, FieldDesc
from runtime.model import EstimatorType

TENSORFLOW = EstimatorType.TENSORFLOW
XGBOOST = EstimatorType.XGBOOST


class TestFeatureColumnCompilation(unittest.TestCase):
    def compile_fc(self, fc, model_type):
        fc_dict = {"feature_columns": [fc]}
        rt_fc_dict = compile_ir_feature_columns(fc_dict, model_type)
        self.assertEqual(len(rt_fc_dict), 1)
        self.assertTrue("feature_columns" in rt_fc_dict)
        fc_list = rt_fc_dict.get("feature_columns")
        self.assertEqual(len(fc_list), 1)
        return fc_list[0]

    def test_numeric_column(self):
        nc = NumericColumn(FieldDesc(name='c1', shape=(2, 3)))

        for model_type in [TENSORFLOW, XGBOOST]:
            compiled_nc = self.compile_fc(nc, model_type)
            self.assertEqual(compiled_nc.key, 'c1')
            self.assertEqual(compiled_nc.shape, (2, 3))

    def test_bucket_column(self):
        nc = NumericColumn(FieldDesc(name='c1', shape=(1, )))
        bc = BucketColumn(nc, (-10, -5, 3, 7))

        for model_type in [TENSORFLOW, XGBOOST]:
            compiled_bc = self.compile_fc(bc, model_type)
            self.assertEqual(compiled_bc.source_column.key, 'c1')
            self.assertEqual(compiled_bc.boundaries, (-10, -5, 3, 7))

    def test_category_id_column(self):
        cc = CategoryIDColumn(FieldDesc(name='c1'), 128)

        for model_type in [TENSORFLOW, XGBOOST]:
            compiled_cc = self.compile_fc(cc, model_type)
            self.assertEqual(compiled_cc.key, 'c1')
            self.assertEqual(compiled_cc.num_buckets, 128)

        cc = CategoryIDColumn(FieldDesc(name='c1', vocabulary=set(['a', 'b'])),
                              128)
        for model_type in [TENSORFLOW, XGBOOST]:
            compiled_cc = self.compile_fc(cc, model_type)
            vocab = sorted(compiled_cc.vocabulary_list)
            self.assertEqual(vocab, ['a', 'b'])

    def test_seq_category_id_column(self):
        scc = SeqCategoryIDColumn(FieldDesc(name='c1'), 64)
        compiled_scc = self.compile_fc(scc, TENSORFLOW)
        # NOTE: TensorFlow SeqCategoryIDColumn does not have key
        # attribute
        # self.assertEqual(compiled_scc.key, 'c1')
        self.assertEqual(compiled_scc.num_buckets, 64)

        with self.assertRaises(AssertionError):
            self.compile_fc(scc, XGBOOST)

    def test_category_hash_column(self):
        chc = CategoryHashColumn(FieldDesc(name='c1', dtype=DataType.STRING),
                                 32)
        for model_type in [TENSORFLOW, XGBOOST]:
            compiled_chc = self.compile_fc(chc, model_type)
            self.assertEqual(compiled_chc.key, 'c1')
            self.assertEqual(compiled_chc.hash_bucket_size, 32)

    def test_cross_column(self):
        cc = CrossColumn(['c1', NumericColumn(FieldDesc(name='c2'))], 4096)
        compiled_cc = self.compile_fc(cc, TENSORFLOW)
        self.assertEqual(list(compiled_cc.keys), ['c1', 'c2'])
        self.assertEqual(compiled_cc.hash_bucket_size, 4096)

        with self.assertRaises(AssertionError):
            self.compile_fc(cc, XGBOOST)

    def test_embedding_column(self):
        chc = CategoryHashColumn(FieldDesc(name='c1', dtype=DataType.STRING),
                                 32)
        ec = EmbeddingColumn(category_column=chc, combiner='sum', dimension=23)

        compiled_ec = self.compile_fc(ec, TENSORFLOW)
        self.assertEqual(compiled_ec.combiner, 'sum')
        self.assertEqual(compiled_ec.dimension, 23)

        compiled_chc = compiled_ec.categorical_column
        self.assertEqual(compiled_chc.key, 'c1')
        self.assertEqual(compiled_chc.hash_bucket_size, 32)

        with self.assertRaises(AssertionError):
            self.compile_fc(ec, XGBOOST)

    def test_indicator_column(self):
        cc = CategoryIDColumn(FieldDesc(name='c1'), 128)
        ic = IndicatorColumn(category_column=cc)

        for model_type in [TENSORFLOW, XGBOOST]:
            compiled_chc = self.compile_fc(ic, model_type)
            compiled_cc = compiled_chc.categorical_column
            self.assertEqual(compiled_cc.key, 'c1')
            self.assertEqual(compiled_cc.num_buckets, 128)


if __name__ == '__main__':
    unittest.main()
