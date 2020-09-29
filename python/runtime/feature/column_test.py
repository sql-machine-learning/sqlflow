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
import runtime.feature.field_desc as fd


class TestFeatureColumn(unittest.TestCase):
    def new_field_desc(self):
        desc = fd.FieldDesc(name="my_feature",
                            dtype=fd.DataType.FLOAT32,
                            delimiter=",",
                            format=fd.DataFormat.CSV,
                            shape=[10],
                            is_sparse=True,
                            vocabulary=["a", "b", "c"])
        return desc

    def check_serialize(self, feature_column):
        d = fc.FeatureColumn.to_dict(feature_column)
        new_fc = fc.FeatureColumn.from_dict_or_feature_column(d)
        new_d = fc.FeatureColumn.to_dict(new_fc)
        typ = type(feature_column)
        self.assertEqual(typ, type(new_fc))
        self.assertEqual(typ.__name__, d["type"])
        self.assertEqual(d, new_d)

        dump_json = json.dumps(feature_column,
                               cls=fc.JSONEncoderWithFeatureColumn)
        new_fc = json.loads(dump_json, cls=fc.JSONDecoderWithFeatureColumn)
        new_d = fc.FeatureColumn.to_dict(new_fc)
        self.assertEqual(type(feature_column), type(new_fc))
        self.assertEqual(typ, type(new_fc))
        self.assertEqual(d, new_d)

    def test_field_desc(self):
        desc = self.new_field_desc()
        json_desc = json.loads(desc.to_json())
        self.assertEqual(json_desc["name"], desc.name)
        self.assertEqual(json_desc["dtype"], desc.dtype)
        self.assertEqual(json_desc["delimiter"], desc.delimiter)
        self.assertEqual(json_desc["format"], desc.format)
        self.assertEqual(json_desc["shape"], desc.shape)
        self.assertEqual(json_desc["is_sparse"], desc.is_sparse)
        vocab = set(json_desc["vocabulary"])
        self.assertEqual(vocab, desc.vocabulary)
        self.assertEqual(json_desc["max_id"], desc.max_id)

    def test_feature_column_subclass(self):
        self.assertTrue(issubclass(fc.NumericColumn, fc.FeatureColumn))
        self.assertTrue(issubclass(fc.BucketColumn, fc.FeatureColumn))
        self.assertTrue(issubclass(fc.EmbeddingColumn, fc.FeatureColumn))
        self.assertTrue(issubclass(fc.IndicatorColumn, fc.FeatureColumn))
        self.assertTrue(issubclass(fc.CategoryColumn, fc.FeatureColumn))
        self.assertTrue(issubclass(fc.CategoryIDColumn, fc.CategoryColumn))
        self.assertTrue(issubclass(fc.CategoryHashColumn, fc.CategoryColumn))
        self.assertTrue(issubclass(fc.SeqCategoryIDColumn, fc.CategoryColumn))
        self.assertTrue(issubclass(fc.CrossColumn, fc.CategoryColumn))

    def test_numeric_column(self):
        desc1 = self.new_field_desc()
        desc2 = self.new_field_desc()

        nc1 = fc.NumericColumn(desc1)
        nc2 = nc1.new_feature_column_from(desc2)
        self.assertTrue(isinstance(nc2, fc.NumericColumn))
        self.assertEqual(len(nc1.get_field_desc()), 1)
        self.assertEqual(len(nc2.get_field_desc()), 1)
        self.assertEqual(nc1.get_field_desc()[0].to_json(),
                         nc2.get_field_desc()[0].to_json())

        d1 = fc.FeatureColumn.to_dict(nc1)
        self.assertEqual(d1["type"], "NumericColumn")
        self.assertEqual(d1["value"]["field_desc"], desc1.to_dict())
        self.check_serialize(nc1)

        d2 = fc.FeatureColumn.to_dict(nc2)
        self.assertEqual(d2["type"], "NumericColumn")
        self.assertEqual(d2["value"]["field_desc"], desc2.to_dict())
        self.check_serialize(nc2)

    def test_bucket_column(self):
        desc = self.new_field_desc()
        nc = fc.NumericColumn(desc)
        boundaries = [-10.5, 20]
        bc = fc.BucketColumn(nc, boundaries)
        self.assertEqual(bc.source_column, nc)
        self.assertEqual(bc.boundaries, boundaries)
        self.assertEqual(bc.num_class(), len(boundaries) + 1)
        self.assertEqual(len(bc.get_field_desc()), 1)
        self.assertEqual(bc.get_field_desc()[0].to_json(), desc.to_json())
        d = fc.FeatureColumn.to_dict(bc)
        self.assertEqual(d["type"], "BucketColumn")
        self.assertEqual(d["value"]["boundaries"], boundaries)
        self.assertEqual(d["value"]["source_column"]["type"], "NumericColumn")
        self.assertEqual(d["value"]["source_column"]["value"]["field_desc"],
                         desc.to_dict())
        self.check_serialize(bc)

        bc = bc.new_feature_column_from(desc)
        self.assertTrue(isinstance(bc, fc.BucketColumn))
        self.assertEqual(bc.boundaries, boundaries)
        self.assertEqual(bc.num_class(), len(boundaries) + 1)
        self.assertEqual(len(bc.get_field_desc()), 1)
        self.assertEqual(bc.get_field_desc()[0].to_json(), desc.to_json())
        d = fc.FeatureColumn.to_dict(bc)
        self.assertEqual(d["type"], "BucketColumn")
        self.assertEqual(d["value"]["boundaries"], boundaries)
        self.assertEqual(d["value"]["source_column"]["type"], "NumericColumn")
        self.assertEqual(d["value"]["source_column"]["value"]["field_desc"],
                         desc.to_dict())
        self.check_serialize(bc)

    def test_category_column(self):
        desc = self.new_field_desc()
        bucket_size = 13

        for fc_class in [
                fc.CategoryIDColumn, fc.CategoryHashColumn,
                fc.SeqCategoryIDColumn
        ]:
            cc = fc_class(desc, bucket_size)
            self.assertEqual(cc.num_class(), bucket_size)
            self.assertEqual(len(cc.get_field_desc()), 1)
            self.assertEqual(cc.get_field_desc()[0].to_json(), desc.to_json())

            d = fc.FeatureColumn.to_dict(cc)
            self.assertEqual(d["type"], fc_class.__name__)
            self.assertEqual(d["value"]["field_desc"], desc.to_dict())
            self.assertEqual(d["value"]["bucket_size"], bucket_size)
            self.check_serialize(cc)

            cc = cc.new_feature_column_from(desc)
            self.assertTrue(isinstance(cc, fc_class))
            self.assertEqual(cc.num_class(), bucket_size)
            self.assertEqual(len(cc.get_field_desc()), 1)
            self.assertEqual(cc.get_field_desc()[0].to_json(), desc.to_json())

            d = fc.FeatureColumn.to_dict(cc)
            self.assertEqual(d["type"], fc_class.__name__)
            self.assertEqual(d["value"]["field_desc"], desc.to_dict())
            self.assertEqual(d["value"]["bucket_size"], bucket_size)
            self.check_serialize(cc)

    def test_weighted_category_column(self):
        desc = self.new_field_desc()
        cc = fc.CategoryIDColumn(desc, 16)

        wcc = fc.WeightedCategoryColumn(cc)
        self.assertEqual(wcc.num_class(), 16)
        self.assertEqual(len(wcc.get_field_desc()), 1)
        d = fc.FeatureColumn.to_dict(wcc)
        self.assertEqual(d["type"], fc.WeightedCategoryColumn.__name__)
        self.assertEqual(d["value"]["category_column"],
                         fc.FeatureColumn.to_dict(cc))
        self.assertEqual(d["value"]["name"], "")
        self.check_serialize(wcc)
        wcc_new = wcc.new_feature_column_from(self.new_field_desc())
        self.assertEqual(d, fc.FeatureColumn.to_dict(wcc_new))

        wcc = fc.WeightedCategoryColumn(name="abc")
        d = fc.FeatureColumn.to_dict(wcc)
        self.assertEqual(d["type"], fc.WeightedCategoryColumn.__name__)
        self.assertEqual(d["value"]["category_column"], None)
        self.assertEqual(d["value"]["name"], "abc")
        self.check_serialize(wcc)
        wcc_new = wcc.new_feature_column_from(self.new_field_desc())
        self.assertEqual(d, fc.FeatureColumn.to_dict(wcc_new))

    def test_cross_column(self):
        desc = self.new_field_desc()
        nc = fc.NumericColumn(desc)
        hash_bucket_size = 1024
        cc = fc.CrossColumn([nc, 'cross_feature_2'], hash_bucket_size)
        self.assertEqual(cc.num_class(), hash_bucket_size)
        descs = cc.get_field_desc()
        self.assertEqual(len(descs), 2)
        self.assertEqual(descs[0].to_json(), desc.to_json())
        self.assertEqual(descs[1].name, 'cross_feature_2')

        d = fc.FeatureColumn.to_dict(cc)
        self.assertEqual(d["type"], "CrossColumn")
        keys = d["value"]["keys"]
        self.assertEqual(len(keys), 2)
        self.assertEqual(keys[0]["type"], "NumericColumn")
        self.assertEqual(keys[0]["value"]["field_desc"], desc.to_dict())
        self.assertEqual(keys[1], "cross_feature_2")
        self.assertEqual(d["value"]["hash_bucket_size"], hash_bucket_size)
        self.check_serialize(cc)

    def test_embedding_and_indicator_column(self):
        desc = self.new_field_desc()
        category_column = fc.CategoryHashColumn(desc, 4096)
        for fc_class in [fc.EmbeddingColumn, fc.IndicatorColumn]:
            fc1 = fc_class(category_column=category_column, name="")
            fc1_descs = fc1.get_field_desc()
            self.assertEqual(len(fc1_descs), 1)
            self.assertEqual(fc1_descs[0].to_json(), desc.to_json())
            fc1 = fc1.new_feature_column_from(desc)
            self.assertTrue(isinstance(fc1, fc_class))
            fc1_descs = fc1.get_field_desc()
            self.assertEqual(len(fc1_descs), 1)
            self.assertEqual(fc1_descs[0].to_json(), desc.to_json())

            d = fc.FeatureColumn.to_dict(fc1)
            self.assertEqual(d["type"], fc_class.__name__)
            self.assertEqual(d["value"]["name"], "")
            self.assertEqual(d["value"]["category_column"]["type"],
                             "CategoryHashColumn")
            self.assertEqual(
                d["value"]["category_column"]["value"]["field_desc"],
                desc.to_dict())
            self.assertEqual(
                d["value"]["category_column"]["value"]["bucket_size"], 4096)
            self.check_serialize(fc1)

            fc2 = fc_class(category_column=None, name="my_category_column")
            fc2_descs = fc2.get_field_desc()
            self.assertEqual(len(fc2_descs), 0)
            fc2 = fc2.new_feature_column_from(desc)
            self.assertTrue(isinstance(fc2, fc_class))
            fc2_descs = fc2.get_field_desc()
            self.assertEqual(len(fc2_descs), 0)

            d = fc.FeatureColumn.to_dict(fc2)
            self.assertEqual(d["type"], fc_class.__name__)
            self.assertEqual(d["value"]["name"], "my_category_column")
            self.assertEqual(d["value"]["category_column"], None)
            self.check_serialize(fc2)


if __name__ == '__main__':
    unittest.main()
