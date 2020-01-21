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
from unittest import TestCase

import tensorflow as tf
from odps import ODPS, tunnel
from sqlflow_submitter.db import (buffered_db_writer, connect,
                                  connect_with_data_source, db_generator,
                                  parseHiveDSN, parseMaxComputeDSN,
                                  parseMySQLDSN)


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
    except:
        field_columns = None

    return field_names, field_columns


class TestDB(TestCase):

    create_statement = "create table test_db (features text, label int)"
    hive_create_statement = 'create table test_db (features string, label int) ROW FORMAT DELIMITED FIELDS TERMINATED BY "\001"'
    select_statement = "select * from test_db"
    drop_statement = "drop table if exists test_db"

    def test_mysql(self):
        driver = os.environ.get('SQLFLOW_TEST_DB')
        if driver == "mysql":
            user = os.environ.get('SQLFLOW_TEST_DB_MYSQL_USER') or "root"
            password = os.environ.get('SQLFLOW_TEST_DB_MYSQL_PASSWD') or "root"
            host = "127.0.0.1"
            port = "3306"
            database = "iris"
            conn = connect(driver,
                           database,
                           user=user,
                           password=password,
                           host=host,
                           port=port)
            self._do_test(driver, conn)

            conn = connect_with_data_source(
                "mysql://root:root@tcp(127.0.0.1:3306)/iris?maxAllowedPacket=0"
            )
            self._do_test(driver, conn)

    def test_hive(self):
        driver = os.environ.get('SQLFLOW_TEST_DB')
        if driver == "hive":
            host = "127.0.0.1"
            port = "10000"
            conn = connect(driver,
                           "iris",
                           user="root",
                           password="root",
                           host=host,
                           port=port)
            self._do_test(driver,
                          conn,
                          hdfs_namenode_addr="127.0.0.1:8020",
                          hive_location="/sqlflow")
            conn.close()

            conn = connect_with_data_source(
                "hive://root:root@127.0.0.1:10000/iris")
            self._do_test(driver, conn)
            self._do_test_hive_specified_db(
                driver,
                conn,
                hdfs_namenode_addr="127.0.0.1:8020",
                hive_location="/sqlflow")
            conn.close()

    def _do_test_hive_specified_db(self,
                                   driver,
                                   conn,
                                   hdfs_namenode_addr="",
                                   hive_location=""):
        create_db = '''create database test_db'''
        create_tbl = '''create table test_db.tbl (features string, label int) ROW FORMAT DELIMITED FIELDS TERMINATED BY "\001"'''
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
    create_statement = "create table test_table_float_fea (features float, label int)"
    drop_statement = "drop table if exists test_table_float_fea"
    insert_statement = "insert into test_table_float_fea (features,label) values(1.0, 0), (2.0, 1)"

    def test_generator(self):
        driver = os.environ.get('SQLFLOW_TEST_DB')
        if driver == "mysql":
            database = "iris"
            user = os.environ.get('SQLFLOW_TEST_DB_MYSQL_USER') or "root"
            password = os.environ.get('SQLFLOW_TEST_DB_MYSQL_PASSWD') or "root"
            conn = connect(driver,
                           database,
                           user=user,
                           password=password,
                           host="127.0.0.1",
                           port="3306")
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
            label_spec = {"feature_name": "label", "shape": []}
            gen = db_generator(driver, conn,
                               "SELECT * FROM test_table_float_fea",
                               ["features"], label_spec, column_name_to_type)
            idx = 0
            for d in gen():
                if idx == 0:
                    self.assertEqual(d, (((1.0, ), ), 0))
                elif idx == 1:
                    self.assertEqual(d, (((2.0, ), ), 1))
                idx += 1
            self.assertEqual(idx, 2)

    def test_generate_fetch_size(self):
        driver = os.environ.get('SQLFLOW_TEST_DB')
        if driver == "mysql":
            database = "iris"
            user = os.environ.get('SQLFLOW_TEST_DB_MYSQL_USER') or "root"
            password = os.environ.get('SQLFLOW_TEST_DB_MYSQL_PASSWD') or "root"
            conn = connect(driver,
                           database,
                           user=user,
                           password=password,
                           host="127.0.0.1",
                           port="3306")
            column_name_to_type = {
                "sepal_length": {
                    "feature_name": "sepal_length",
                    "delimiter": "",
                    "dtype": "float32",
                    "is_sparse": False,
                    "shape": []
                }
            }
            label_spec = {"feature_name": "label", "shape": []}
            gen = db_generator(driver,
                               conn,
                               'SELECT * FROM iris.train limit 10',
                               ["sepal_length"],
                               label_spec,
                               column_name_to_type,
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
            parseHiveDSN(
                "usr:pswd@hiveserver:1000/mydb?auth=PLAIN&session.mapreduce_job_quenename=mr"
            ))
        self.assertEqual(
            ("usr", "pswd", "hiveserver", "1000", "my_db", "PLAIN", {
                "mapreduce_job_quenename": "mr"
            }),
            parseHiveDSN(
                "usr:pswd@hiveserver:1000/my_db?auth=PLAIN&session.mapreduce_job_quenename=mr"
            ))
        self.assertEqual(
            ("root", "root", "127.0.0.1", None, "mnist", "PLAIN", {}),
            parseHiveDSN("root:root@127.0.0.1/mnist?auth=PLAIN"))
        self.assertEqual(("root", "root", "127.0.0.1", None, None, "", {}),
                         parseHiveDSN("root:root@127.0.0.1"))

    def test_parse_maxcompute_dsn(self):
        self.assertEqual(
            ("access_id", "access_key", "http://maxcompute-service.com/api",
             "test_ci"),
            parseMaxComputeDSN(
                "access_id:access_key@maxcompute-service.com/api?curr_project=test_ci&scheme=http"
            ))
