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
from odps import tunnel
from runtime.db import (MYSQL_FIELD_TYPE_DICT, buffered_db_writer, connect,
                        connect_with_data_source, db_generator,
                        get_table_schema, limit_select, parseHiveDSN,
                        parseMaxComputeDSN, parseMySQLDSN, read_feature,
                        read_features_from_row, selected_columns_and_types)


def _execute_maxcompute(conn, statement):
    compress = tunnel.CompressOption.CompressAlgorithm.ODPS_ZLIB
    inst = conn.execute_sql(statement)
    if not inst.is_successful():
        return None, None

    r = inst.open_reader(tunnel=True, compress_option=compress)
    field_names = [col.name for col in r._schema.columns]
    rows = [[v[1] for v in rec] for rec in r[0:r.count]]
    return field_names, list(map(list, zip(*rows))) if r.count > 0 else None


def execute(driver, conn, statement):
    if driver == "maxcompute":
        return _execute_maxcompute(conn, statement)

    cursor = conn.cursor()
    cursor.execute(statement)
    if driver == "hive":
        field_names = None if cursor.description is None \
            else [i[0][i[0].find('.') + 1:] for i in cursor.description]
    else:
        field_names = None if cursor.description is None \
            else [i[0] for i in cursor.description]

    try:
        rows = cursor.fetchall()
        field_columns = list(map(list, zip(*rows))) if len(rows) > 0 else None
    except:  # noqa: E722
        field_columns = None

    return field_names, field_columns


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
        driver = testing.get_driver()
        user, password, host, port, database, _ = parseMySQLDSN(
            testing.get_mysql_dsn())
        conn = connect(driver,
                       database,
                       user=user,
                       password=password,
                       host=host,
                       port=port)
        self._do_test(driver, conn)
        conn.close()

        conn = testing.get_singleton_db_connection()
        self._do_test(driver, conn)

    @unittest.skipUnless(testing.get_driver() == "hive", "skip non hive tests")
    def test_hive(self):
        driver = testing.get_driver()
        user, password, host, port, database, _, _ = parseHiveDSN(
            testing.get_hive_dsn())
        conn = connect(driver,
                       database,
                       user=user,
                       password=password,
                       host=host,
                       port=port)
        self._do_test(driver,
                      conn,
                      hdfs_namenode_addr="127.0.0.1:8020",
                      hive_location="/sqlflow")
        conn.close()

        conn = testing.get_singleton_db_connection()
        self._do_test(driver, conn)
        self._do_test_hive_specified_db(driver,
                                        conn,
                                        hdfs_namenode_addr="127.0.0.1:8020",
                                        hive_location="/sqlflow")

    def _do_test_hive_specified_db(self,
                                   driver,
                                   conn,
                                   hdfs_namenode_addr="",
                                   hive_location=""):
        create_db = '''create database if not exists test_db'''
        create_tbl = '''create table test_db.tbl (features string, label int)
                        ROW FORMAT DELIMITED FIELDS TERMINATED BY "\001"'''
        drop_tbl = '''drop table if exists test_db.tbl'''
        select_tbl = '''select * from test_db.tbl'''
        table_schema = ["label", "features"]
        values = [(1, '5,6,1,2')] * 10
        execute(driver, conn, create_db)
        execute(driver, conn, drop_tbl)
        execute(driver, conn, create_tbl)
        with buffered_db_writer(driver,
                                conn,
                                "test_db.tbl",
                                table_schema,
                                buff_size=10,
                                hdfs_namenode_addr=hdfs_namenode_addr,
                                hive_location=hive_location) as w:
            for row in values:
                w.write(row)

        field_names, data = execute(driver, conn, select_tbl)

        expect_features = ['5,6,1,2'] * 10
        expect_labels = [1] * 10

        self.assertEqual(field_names, ['features', 'label'])
        self.assertEqual(expect_features, data[0])
        self.assertEqual(expect_labels, data[1])

    def _do_test(self, driver, conn, hdfs_namenode_addr="", hive_location=""):
        table_name = "test_db"
        table_schema = ["label", "features"]
        values = [(1, '5,6,1,2')] * 10

        execute(driver, conn, self.drop_statement)

        if driver == "hive":
            execute(driver, conn, self.hive_create_statement)
        else:
            execute(driver, conn, self.create_statement)
        with buffered_db_writer(driver,
                                conn,
                                table_name,
                                table_schema,
                                buff_size=10,
                                hdfs_namenode_addr=hdfs_namenode_addr,
                                hive_location=hive_location) as w:
            for row in values:
                w.write(row)

        field_names, data = execute(driver, conn, self.select_statement)

        expect_features = ['5,6,1,2'] * 10
        expect_labels = [1] * 10

        self.assertEqual(field_names, ['features', 'label'])
        self.assertEqual(expect_features, data[0])
        self.assertEqual(expect_labels, data[1])


class TestGenerator(TestCase):
    create_statement = "create table test_table_float_fea " \
                       "(features float, label int)"
    drop_statement = "drop table if exists test_table_float_fea"
    insert_statement = "insert into test_table_float_fea (features,label)" \
                       " values(1.0, 0), (2.0, 1)"

    @unittest.skipUnless(testing.get_driver() == "mysql",
                         "skip non mysql tests")
    def test_generator(self):
        driver = testing.get_driver()
        user, password, host, port, database, _ = parseMySQLDSN(
            testing.get_mysql_dsn())
        conn = connect(driver,
                       database,
                       user=user,
                       password=password,
                       host=host,
                       port=int(port))
        # prepare test data
        execute(driver, conn, self.drop_statement)
        execute(driver, conn, self.create_statement)
        execute(driver, conn, self.insert_statement)

        column_name_to_type = {
            "features": {
                "feature_name": "features",
                "delimiter": "",
                "dtype": "float32",
                "is_sparse": False,
                "shape": []
            }
        }
        label_meta = {"feature_name": "label", "shape": [], "delimiter": ""}
        gen = db_generator(conn, "SELECT * FROM test_table_float_fea",
                           label_meta)
        idx = 0
        for row, label in gen():
            features = read_features_from_row(row, ["features"], ["features"],
                                              column_name_to_type)
            d = (features, label)
            if idx == 0:
                self.assertEqual(d, (((1.0, ), ), 0))
            elif idx == 1:
                self.assertEqual(d, (((2.0, ), ), 1))
            idx += 1
        self.assertEqual(idx, 2)

    @unittest.skipUnless(testing.get_driver() == "mysql",
                         "skip non mysql tests")
    def test_generate_fetch_size(self):
        label_meta = {"feature_name": "label", "shape": [], "delimiter": ""}
        gen = db_generator(testing.get_singleton_db_connection(),
                           'SELECT * FROM iris.train limit 10',
                           label_meta,
                           fetch_size=4)
        self.assertEqual(len([g for g in gen()]), 10)


class TestConnectWithDataSource(TestCase):
    def test_parse_mysql_dsn(self):
        # [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
        self.assertEqual(("usr", "pswd", "localhost", "8000", "mydb", {
            "param1": "value1"
        }), parseMySQLDSN("usr:pswd@tcp(localhost:8000)/mydb?param1=value1"))

    def test_parse_hive_dsn(self):
        self.assertEqual(
            ("usr", "pswd", "hiveserver", "1000", "mydb", "PLAIN", {
                "mapreduce_job_quenename": "mr"
            }),
            parseHiveDSN("usr:pswd@hiveserver:1000/mydb?auth=PLAIN&"
                         "session.mapreduce_job_quenename=mr"))
        self.assertEqual(
            ("usr", "pswd", "hiveserver", "1000", "my_db", "PLAIN", {
                "mapreduce_job_quenename": "mr"
            }),
            parseHiveDSN("usr:pswd@hiveserver:1000/my_db?auth=PLAIN&"
                         "session.mapreduce_job_quenename=mr"))
        self.assertEqual(
            ("root", "root", "127.0.0.1", None, "mnist", "PLAIN", {}),
            parseHiveDSN("root:root@127.0.0.1/mnist?auth=PLAIN"))
        self.assertEqual(("root", "root", "127.0.0.1", None, None, "", {}),
                         parseHiveDSN("root:root@127.0.0.1"))

    def test_parse_maxcompute_dsn(self):
        self.assertEqual(("access_id", "access_key",
                          "http://maxcompute-service.com/api", "test_ci"),
                         parseMaxComputeDSN(
                             "access_id:access_key@maxcompute-service.com/api?"
                             "curr_project=test_ci&scheme=http"))

    def test_kv_feature_column(self):
        feature_spec = {
            "name": "kv_feature_name",
            "is_sparse": True,
            "format": "kv",
            "dtype": "float",
            "shape": [10],
        }

        raw_val = "0:1 3:4 4:6"
        indices, values, shape = read_feature(raw_val, feature_spec,
                                              feature_spec["name"])
        self.assertTrue(np.array_equal(indices, np.array([0, 3, 4],
                                                         dtype=int)))
        self.assertTrue(np.array_equal(values, np.array([1, 4, 6], dtype=int)))
        self.assertTrue(np.array_equal(shape, np.array([10], dtype='float')))


class TestGetTableSchema(TestCase):
    def test_get_table_schema(self):
        driver = testing.get_driver()
        conn = testing.get_singleton_db_connection()
        if driver == "mysql":
            schema = get_table_schema(conn, "iris.train")
            expect = (
                ('sepal_length', 'FLOAT'),
                ('sepal_width', 'FLOAT'),
                ('petal_length', 'FLOAT'),
                ('petal_width', 'FLOAT'),
                ('class', 'INT(11)'),
            )
            self.assertTrue(np.array_equal(expect, schema))

            schema = selected_columns_and_types(
                conn,
                "SELECT sepal_length, petal_width * 2.3 new_petal_width, "
                "class FROM iris.train")
            expect = [
                ("sepal_length", "FLOAT"),
                ("new_petal_width", "DOUBLE"),
                ("class", "INT"),
            ]
            self.assertTrue(np.array_equal(expect, schema))
        elif driver == "hive":
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
        elif driver == "maxcompute":
            case_db = os.getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
            table = "%s.sqlflow_test_iris_train" % case_db
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

        addr = os.getenv("SQLFLOW_TEST_DB_MYSQL_ADDR", "localhost:3306")
        conn = connect_with_data_source(
            "mysql://root:root@tcp(%s)/?maxAllowedPacket=0" % addr)
        cursor = conn.cursor()

        table_name = "iris.test_mysql_field_type_table"
        drop_table_sql = "DROP TABLE IF EXISTS %s" % table_name
        create_table_sql = "CREATE TABLE IF NOT EXISTS " + \
                           table_name + "(a %s)"
        select_sql = "SELECT * FROM %s" % table_name

        for int_type, str_type in MYSQL_FIELD_TYPE_DICT.items():
            if str_type in ["VARCHAR", "CHAR"]:
                str_type += "(255)"

            cursor.execute(drop_table_sql)
            cursor.execute(create_table_sql % str_type)
            cursor.execute(select_sql)

            int_type_actual = cursor.description[0][1]
            cursor.execute(drop_table_sql)

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


if __name__ == "__main__":
    unittest.main()
