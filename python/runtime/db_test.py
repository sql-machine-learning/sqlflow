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
from unittest import TestCase

import numpy as np
import runtime.testing as testing
from runtime.db import (XGBOOST_NULL_MAGIC, buffered_db_writer,
                        connect_with_data_source, db_generator,
                        get_table_schema, limit_select, read_feature,
                        read_features_from_row, selected_columns_and_types)
from runtime.dbapi import connect
from runtime.dbapi.mysql import MYSQL_FIELD_TYPE_DICT


def execute(conn, statement):
    rs = conn.query(statement)
    field_names = [c[0] for c in rs.column_info()]
    rows = [r for r in rs]
    return field_names, rows


class TestDB(TestCase):

    create_statement = "create table test_db (features text, label int)"
    hive_create_statement = 'create table test_db (features string, ' \
                            'label int) ROW FORMAT DELIMITED FIELDS ' \
                            'TERMINATED BY "\001"'
    select_statement = "select * from test_db"
    drop_statement = "drop table if exists test_db"

    @unittest.skipUnless(testing.get_driver() == "mysql",
                         "skip non mysql tests")
    def test_mysql(self):
        conn = connect(testing.get_datasource())
        self._do_test(conn)
        conn.close()

    @unittest.skipUnless(testing.get_driver() == "hive", "skip non hive tests")
    def test_hive(self):
        uri = testing.get_datasource()
        conn = connect(uri)
        self._do_test(conn)
        self._do_test_hive_specified_db(conn)

    def _do_test_hive_specified_db(self, conn):
        create_db = '''create database if not exists test_db'''
        create_tbl = '''create table test_db.tbl (features string, label int)
                        ROW FORMAT DELIMITED FIELDS TERMINATED BY "\001"'''
        drop_tbl = '''drop table if exists test_db.tbl'''
        select_tbl = '''select * from test_db.tbl'''
        table_schema = ["label", "features"]
        values = [(1, '5,6,1,2')] * 10
        self.assertTrue(conn.execute(create_db))
        self.assertTrue(conn.execute(drop_tbl))
        self.assertTrue(conn.execute(create_tbl))

        with buffered_db_writer(conn,
                                "test_db.tbl",
                                table_schema,
                                buff_size=10) as w:
            for row in values:
                w.write(row)

        field_names, data = execute(conn, select_tbl)

        expect_result = [('5,6,1,2', 1)] * 10

        self.assertEqual(field_names, ['features', 'label'])
        self.assertEqual(expect_result, data)

    def _do_test(self, conn):
        table_name = "test_db"
        table_schema = ["features", "label"]
        values = [('5,6,1,2', 1)] * 10

        conn.execute(self.drop_statement)

        if conn.driver == "hive":
            conn.execute(self.hive_create_statement)
        else:
            conn.execute(self.create_statement)
        with buffered_db_writer(conn, table_name, table_schema,
                                buff_size=10) as w:
            for row in values:
                w.write(row)

        field_names, data = execute(conn, self.select_statement)

        self.assertEqual(table_schema, field_names)
        self.assertEqual(values, data)


class TestGenerator(TestCase):
    create_statement = """create table test_table_float_fea
(f1 float, f2 int, f3str VARCHAR(255),
f4sparse VARCHAR(255), f5dense VARCHAR(255), label int)"""
    drop_statement = "drop table if exists test_table_float_fea"
    insert_statement = """insert into test_table_float_fea
(f1,f2,f3str,f4sparse,f5dense,label)
values(1.0,1,'a','1:1.0 2:2.0','1,2,3',0), (NULL,NULL,NULL,NULL,'1,2,3',1)"""

    @unittest.skipUnless(testing.get_driver() == "mysql",
                         "skip non mysql tests")
    def test_generator(self):
        conn = connect(testing.get_datasource())
        # prepare test data
        conn.execute(self.drop_statement)
        conn.execute(self.create_statement)
        conn.execute(self.insert_statement)

        column_name_to_type = {
            "f1": {
                "feature_name": "f1",
                "delimiter": "",
                "dtype": "float32",
                "is_sparse": False,
                "shape": []
            },
            "f2": {
                "feature_name": "f2",
                "delimiter": "",
                "dtype": "int64",
                "is_sparse": False,
                "shape": []
            },
            "f3str": {
                "feature_name": "f3str",
                "delimiter": "",
                "dtype": "string",
                "is_sparse": False,
                "shape": []
            },
            "f4sparse": {
                "feature_name": "f4sparse",
                "delimiter": "",
                "dtype": "float32",
                "is_sparse": True,
                "shape": [],
                "format": "kv"
            },
            "f5dense": {
                "feature_name": "f5dense",
                "delimiter": ",",
                "dtype": "int64",
                "is_sparse": False,
                "shape": [3]
            }
        }
        label_meta = {"feature_name": "label", "shape": [], "delimiter": ""}
        gen = db_generator(conn, "SELECT * FROM test_table_float_fea",
                           label_meta)
        idx = 0
        for row, label in gen():
            if idx == 0:
                features = read_features_from_row(
                    row, ["f1", "f2", "f3str", "f4sparse", "f5dense"],
                    ["f1", "f2", "f3str", "f4sparse", "f5dense"],
                    column_name_to_type)
                self.assertEqual(1.0, features[0][0])
                self.assertEqual(1, features[1][0])
                self.assertEqual('a', features[2][0])
                self.assertTrue(
                    np.array_equal(np.array([[1], [2]]), features[3][0]))
                self.assertTrue(
                    np.array_equal(np.array([1., 2.], dtype=np.float32),
                                   features[3][1]))
                self.assertTrue(
                    np.array_equal(np.array([1, 2, 3]), features[4][0]))
                self.assertEqual(0, label)
            elif idx == 1:
                try:
                    features = read_features_from_row(
                        row, ["f1", "f2", "f3str", "f4sparse", "f5dense"],
                        ["f1", "f2", "f3str", "f4sparse", "f5dense"],
                        column_name_to_type)
                except Exception as e:
                    self.assertTrue(isinstance(e, ValueError))
                features = read_features_from_row(
                    row, ["f1", "f2", "f3str", "f4sparse", "f5dense"],
                    ["f1", "f2", "f3str", "f4sparse", "f5dense"],
                    column_name_to_type,
                    is_xgboost=True)
                self.assertEqual(XGBOOST_NULL_MAGIC, features[0][0])
                self.assertEqual(int(XGBOOST_NULL_MAGIC), features[1][0])
                self.assertEqual("", features[2][0])
                self.assertTrue(np.array_equal(np.array([]), features[3][0]))
                self.assertTrue(np.array_equal(np.array([]), features[3][1]))
                self.assertTrue(
                    np.array_equal(np.array([1, 2, 3]), features[4][0]))
                self.assertEqual(1, label)
            idx += 1
        self.assertEqual(idx, 2)

    @unittest.skipUnless(testing.get_driver() == "mysql",
                         "skip non mysql tests")
    def test_generate_fetch_size(self):
        label_meta = {"feature_name": "label", "shape": [], "delimiter": ""}
        gen = db_generator(testing.get_singleton_db_connection(),
                           'SELECT * FROM iris.train limit 10', label_meta)
        self.assertEqual(len([g for g in gen()]), 10)


class TestConnectWithDataSource(TestCase):
    def test_kv_feature_column(self):
        feature_spec = {
            "name": "kv_feature_name",
            "is_sparse": True,
            "format": "kv",
            "dtype": "float32",
            "shape": [10],
            "delimiter": ""
        }

        raw_val = "0:1 3:4 4:6"
        indices, values, shape = read_feature(raw_val, feature_spec,
                                              feature_spec["name"], True)
        self.assertTrue(
            np.array_equal(indices, np.array([0, 3, 4], dtype='int64')))
        self.assertTrue(
            np.array_equal(values, np.array([1, 4, 6], dtype='float32')))
        self.assertTrue(np.array_equal(shape, np.array([10], dtype='float32')))


class TestGetTableSchema(TestCase):
    def test_get_table_schema(self):
        conn = testing.get_singleton_db_connection()
        if conn.driver == "mysql":
            schema = get_table_schema(conn, "iris.train")
            expect = [
                ('sepal_length', 'FLOAT'),
                ('sepal_width', 'FLOAT'),
                ('petal_length', 'FLOAT'),
                ('petal_width', 'FLOAT'),
                ('class', 'INT'),
            ]
            self.assertEqual(expect, schema)

            schema = selected_columns_and_types(
                conn,
                "SELECT sepal_length, petal_width * 2.3 new_petal_width, "
                "class FROM iris.train")
            expect = [
                ("sepal_length", "FLOAT"),
                ("new_petal_width", "DOUBLE"),
                ("class", "INT"),
            ]
            self.assertEqual(expect, schema)
        elif conn.driver == "hive":
            schema = get_table_schema(conn, "iris.train")
            expect = (
                ('sepal_length', 'FLOAT'),
                ('sepal_width', 'FLOAT'),
                ('petal_length', 'FLOAT'),
                ('petal_width', 'FLOAT'),
                ('class', 'INT'),
            )
            self.assertTrue(np.array_equal(expect, schema))

            schema = selected_columns_and_types(
                conn,
                "SELECT sepal_length, petal_width * 2.3 AS new_petal_width, "
                "class FROM iris.train")
            expect = [
                ("sepal_length", "FLOAT"),
                ("new_petal_width", "FLOAT"),
                ("class", "INT"),
            ]
            self.assertTrue(np.array_equal(expect, schema))
        elif conn.driver == "maxcompute":
            case_db = os.getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
            table = "%s.sqlflow_iris_train" % case_db
            schema = get_table_schema(conn, table)
            expect = [
                ('sepal_length', 'DOUBLE'),
                ('sepal_width', 'DOUBLE'),
                ('petal_length', 'DOUBLE'),
                ('petal_width', 'DOUBLE'),
                ('class', 'BIGINT'),
            ]
            self.assertTrue(np.array_equal(expect, schema))

            schema = selected_columns_and_types(
                conn,
                "SELECT sepal_length, petal_width * 2.3 new_petal_width, "
                "class FROM %s" % table)
            expect = [
                ("sepal_length", "DOUBLE"),
                ("new_petal_width", "DOUBLE"),
                ("class", "BIGINT"),
            ]
            self.assertTrue(np.array_equal(expect, schema))


class TestMySQLFieldType(TestCase):
    @unittest.skipUnless(
        os.getenv("SQLFLOW_TEST_DB") == "mysql", "run only in mysql")
    def test_field_type(self):
        self.assertGreater(len(MYSQL_FIELD_TYPE_DICT), 0)

        conn = connect_with_data_source(testing.get_datasource())

        table_name = "iris.test_mysql_field_type_table"
        drop_table_sql = "DROP TABLE IF EXISTS %s" % table_name
        create_table_sql = "CREATE TABLE IF NOT EXISTS " + \
                           table_name + "(a %s)"
        select_sql = "SELECT * FROM %s" % table_name

        for int_type, str_type in MYSQL_FIELD_TYPE_DICT.items():
            if str_type in ["VARCHAR", "CHAR"]:
                str_type += "(255)"

            conn.execute(drop_table_sql)
            conn.execute(create_table_sql % str_type)
            # we are meant to use low layer cursor here to
            # check the type value with the real value returned by mysql
            cursor = conn.cursor()
            cursor.execute(select_sql)
            int_type_actual = cursor.description[0][1]
            cursor.close()
            conn.execute(drop_table_sql)

            self.assertEqual(int_type_actual, int_type,
                             "%s not match" % str_type)


class TestLimitSelect(TestCase):
    def test_limit_select(self):
        self.assertEqual("SELECT * FROM t LIMIT 2",
                         limit_select("SELECT * FROM t LIMIT 30", 2))

        self.assertEqual("SELECT * FROM t LIMIT 30; \t",
                         limit_select("SELECT * FROM t LIMIT 30; \t", 100))

        self.assertEqual("SELECT * FROM t LIMIT 3",
                         limit_select("SELECT * FROM t", 3))

        self.assertEqual("SELECT * FROM t \t  LIMIT 4; ",
                         limit_select("SELECT * FROM t \t ; ", 4))


@unittest.skipIf(testing.get_driver() == "maxcompute", "skip maxcompute tests")
class TestQuery(TestCase):
    def test_query(self):
        conn = connect_with_data_source(testing.get_datasource())
        rs = conn.query("select * from iris.train limit 1")
        rows = [row for row in rs]
        self.assertEqual(1, len(rows))

        conn.execute("drop table if exists A")
        conn.execute("create table A(a int);")
        conn.execute("insert into A values(1)")
        rs = conn.query("select * from A;")
        rows = [row for row in rs]
        self.assertEqual(1, len(rows))

        conn.query("truncate table A")
        rs = conn.query("select * from A;")
        rows = [row for row in rs]
        self.assertEqual(0, len(rows))
        columns = rs.column_info()
        self.assertEqual(1, len(columns))
        self.assertEqual("a", columns[0][0])
        self.assertEqual("INT", columns[0][1])

        self.assertTrue(conn.execute("drop table if exists A"))


if __name__ == "__main__":
    unittest.main()
