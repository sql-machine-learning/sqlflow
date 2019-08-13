# Copyright 2019 The SQLFlow Authors. All rights reserved.
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
import os
from unittest import TestCase

from sqlflow_submitter.xgboost.sqlflow_data_source import SQLFlowDSConfig, SQLFlowDataSource
from launcher import config_helper, config_fields, register_data_source, XGBoostRecord, XGBoostResult
from launcher.data_source import create_data_source_init_fn
from sqlflow_submitter.db_test import execute as db_exec
from sqlflow_submitter.db import connect, insert_values


class TestSQLFlowDataSource(TestCase):
    def setUp(self) -> None:
        register_data_source('sqlflow', SQLFlowDSConfig, SQLFlowDataSource)

        db_conf = {
            'driver': 'mysql',
            'database': "iris",
            'user': os.environ.get('SQLFLOW_TEST_DB_MYSQL_USER') or "root",
            'password': os.environ.get('SQLFLOW_TEST_DB_MYSQL_PASSWD') or "root",
            'host': '127.0.0.1',
            'port': '3306',
        }

        def mk_ds_conf(is_train):
            return {
                'db_config': db_conf,
                'standard_select': 'select * from input_table',
                'is_tf_integrated': False,
                'is_train': is_train,
                'x': [],
                'output_table': None if is_train else 'output_table',
            }

        train_ds_conf = mk_ds_conf(True)
        pred_ds_conf = mk_ds_conf(False)

        def mk_col_conf(is_train):
            col_conf = {
                'features': {'columns': 'f1,f3,f2', 'feature_num': 3},
            }
            if is_train:
                col_conf['label'] = 'label1'
                col_conf['group'] = 'group1'
                col_conf['weight'] = 'weight1'
            else:
                col_conf['append_columns'] = ['a1', 'a2', 'f3']
                col_conf['result_columns'] = {
                    'result_column': 'result',
                    'probability_column': 'prob',
                    'detail_column': 'detail',
                    'leaf_column': 'leaf'
                }
            return col_conf

        train_col_conf = mk_col_conf(True)
        pred_col_conf = mk_col_conf(False)

        conn = connect(**db_conf)

        def run_sql(statement):
            return db_exec(db_conf['driver'], conn, statement)

        run_sql("DROP TABLE IF EXISTS input_table")
        run_sql("""
        CREATE TABLE input_table(
        f1 float, f2 float, f3 float,
        label1 int, weight1 float, group1 int,
        a1 varchar(10), a2 varchar(10) 
        )""")
        self._data = [1.1, 2.2, 3.3, 4.5, 1, 2, 'A1', "A2"]
        data = [self._data[:] for _ in range(100)]
        schema = ['f1', 'f2', 'f3', 'weight1', 'label1', 'group1', 'a1', 'a2']
        insert_values(db_conf['driver'], conn, 'input_table', schema, data)
        self._sql = run_sql

        run_sql("DROP TABLE IF EXISTS output_table")
        run_sql("""
        CREATE TABLE output_table(
        result float, prob float, detail VARCHAR(100), leaf VARCHAR(100),
        a1 varchar(10), a2 varchar(10), f3 float
        )""")
        self._ret = XGBoostResult(
            result=1.0,
            classification_prob=0.8,
            classification_detail=[0.15, 0.8, 0.05],
            leaf_indices=[1, 2, 3, 4, 5],
            append_info=["A1", "A2", 3.3])

        def build_ds(is_train):
            ds_conf = train_ds_conf if is_train else pred_ds_conf
            col_conf = train_col_conf if is_train else pred_col_conf
            ds_fields = config_helper.load_config(
                config_fields.DataSourceFields,
                **{'name': "sqlflow", 'config': ds_conf})
            init_fn = create_data_source_init_fn(ds_fields)
            col_fields = config_helper.load_config(config_fields.ColumnFields, **col_conf)
            return init_fn(0, 1, col_fields)

        self._ds_builder = build_ds

    def test_build(self):
        if os.environ.get('SQLFLOW_TEST_DB') != "mysql":
            return

        ds = self._ds_builder(True)
        assert isinstance(ds, SQLFlowDataSource)
        assert ds._label_offset == 3
        assert ds._group_offset == 4
        assert ds._weight_offset == 5

        ds = self._ds_builder(False)
        assert isinstance(ds, SQLFlowDataSource)
        assert ds._append_indices == [3, 4, 1]

    def test_read(self):
        if os.environ.get('SQLFLOW_TEST_DB') != "mysql":
            return

        ds = self._ds_builder(True)
        i = 0
        for rcd in ds.read():
            assert rcd.indices == [0, 1, 2]
            # order of feature columns: [f1, f3, f2]
            assert rcd.values == [self._data[0], self._data[2], self._data[1]]
            assert rcd.weight == self._data[3]
            assert rcd.label == self._data[4]
            assert rcd.group == self._data[5]
            i += 1
        assert i == 100

        ds = self._ds_builder(False)
        i = 0
        for rcd in ds.read():
            assert rcd.indices == [0, 1, 2]
            # order of feature columns: [f1, f3, f2]
            assert rcd.values == [self._data[0], self._data[2], self._data[1]]
            # append columns: [a1, a2, f3]
            assert rcd.append_info == [self._data[-2], self._data[-1], self._data[2]]
            i += 1
        assert i == 100

    def test_write(self):
        if os.environ.get('SQLFLOW_TEST_DB') != "mysql":
            return

        ds = self._ds_builder(False)
        ds.write((self._ret for _ in range(100)))

        indices, cols = self._sql("select * from output_table")
        app_fields = ds._result_schema['append_columns']
        for idx, c_values in zip(*(indices, cols)):
            for c_val in c_values:
                if idx == 'result':
                    assert c_val == self._ret.result
                elif idx == 'prob':
                    assert c_val == self._ret.classification_prob
                elif idx == 'detail':
                    detail = [(k, v) for k, v in json.loads(c_val).items()]
                    sorted(detail, key=lambda x: x[0])
                    detail = [v for _, v in detail]
                    assert detail == self._ret.classification_detail
                elif idx == 'leaf':
                    assert list(map(int, c_val.split(','))) == self._ret.leaf_indices
                if idx in app_fields:
                    assert c_val == self._ret.append_info[app_fields.index(idx)]
