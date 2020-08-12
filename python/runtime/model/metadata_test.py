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

import os
import unittest

from runtime.feature.column import NumericColumn
from runtime.feature.field_desc import FieldDesc
from runtime.model.metadata import (collect_metadata, load_metadata,
                                    save_metadata)


class TestMetadata(unittest.TestCase):
    def setUp(self):
        self.file_name = 'meta.json'

    def tearDown(self):
        if os.path.exists(self.file_name):
            os.remove(self.file_name)

    def test_metadata(self):
        original_sql = '''
        SELECT c1, c2, class FROM my_db.train_table
        TO TRAIN my_docker_image:latest/DNNClassifier
        WITH
            model.n_classes = 3,
            model.hidden_units = [16, 32],
            validation.select="SELECT c1, c2, class FROM my_db.val_table"
        INTO my_db.my_dnn_model;
        '''

        select = "SELECT c1, c2, class FROM my_db.train_table"
        validation_select = "SELECT c1, c2, class FROM my_db.val_table"
        model_repo_image = "my_docker_image:latest"
        estimator = "DNNClassifier"
        attributes = {
            'n_classes': 3,
            'hidden_units': [16, 32],
        }

        features = {
            'feature_columns': [
                NumericColumn(FieldDesc(name='c1', shape=[3], delimiter=",")),
                NumericColumn(FieldDesc(name='c2', shape=[1])),
            ],
        }

        label = NumericColumn(FieldDesc(name='class', shape=[5],
                                        delimiter=','))

        def check_metadata(meta):
            self.assertEqual(meta['original_sql'], original_sql)
            self.assertEqual(meta['select'], select)
            self.assertEqual(meta['validation_select'], validation_select)
            self.assertEqual(meta['model_repo_image'], model_repo_image)
            self.assertEqual(meta['class_name'], estimator)
            self.assertEqual(meta['attributes'], attributes)
            meta_features = meta['features']
            meta_label = meta['label']
            self.assertEqual(len(meta_features), 1)
            self.assertEqual(len(meta_features['feature_columns']), 2)
            meta_features = meta_features['feature_columns']
            self.assertEqual(type(meta_features[0]), NumericColumn)
            self.assertEqual(type(meta_features[1]), NumericColumn)
            field_desc = meta_features[0].get_field_desc()[0]
            self.assertEqual(field_desc.name, 'c1')
            self.assertEqual(field_desc.shape, [3])
            self.assertEqual(field_desc.delimiter, ',')
            field_desc = meta_features[1].get_field_desc()[0]
            self.assertEqual(field_desc.name, 'c2')
            self.assertEqual(field_desc.shape, [1])
            self.assertEqual(type(meta_label), NumericColumn)
            field_desc = meta_label.get_field_desc()[0]
            self.assertEqual(field_desc.name, 'class')
            self.assertEqual(field_desc.shape, [5])
            self.assertEqual(field_desc.delimiter, ',')
            self.assertEqual(meta['evaluation'], {'accuracy': 0.5})
            self.assertEqual(meta['my_data'], 0.25)

        meta = collect_metadata(original_sql,
                                select,
                                validation_select,
                                model_repo_image,
                                estimator,
                                attributes,
                                features,
                                label, {'accuracy': 0.5},
                                my_data=0.25)

        check_metadata(meta)

        save_metadata(self.file_name, meta)
        meta = load_metadata(self.file_name)
        check_metadata(meta)


if __name__ == '__main__':
    unittest.main()
