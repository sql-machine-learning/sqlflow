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

import runtime.db as db


def get_driver():
    return os.getenv("SQLFLOW_TEST_DB")


def get_submitter():
    return os.getenv("SQLFLOW_submitter")


def get_mysql_dsn():
    usr = os.getenv("SQLFLOW_TEST_DB_MYSQL_USER", "root")
    pwd = os.getenv("SQLFLOW_TEST_DB_MYSQL_PASSWD", "root")
    net = os.getenv("SQLFLOW_TEST_DB_MYSQL_NET", "tcp")
    addr = os.getenv("SQLFLOW_TEST_DB_MYSQL_ADDR", "127.0.0.1:3306")
    return "%s:%s@%s(%s)/iris?maxAllowedPacket=0" % (usr, pwd, net, addr)


def get_hive_dsn():
    return ("root:root@localhost:10000/iris?"
            "hdfs_namenode_addr=localhost:8020&hive_location=/sqlflow")


def get_maxcompute_dsn():
    ak = os.getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_AK")
    if not ak:
        raise ValueError("SQLFLOW_TEST_DB_MAXCOMPUTE_AK must be set")

    sk = os.getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_SK")
    if not sk:
        raise ValueError("SQLFLOW_TEST_DB_MAXCOMPUTE_SK must be set")

    project = os.getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
    if not project:
        raise ValueError("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT must be set")

    endpoint = os.getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_ENDPOINT",
                         "http://service-maxcompute.com/api")

    scheme = None
    if endpoint.startswith("http://"):
        scheme = "http"
    elif endpoint.startswith("https://"):
        scheme = "https"

    if scheme:
        endpoint = endpoint[len(scheme) + 3:]

    params = {}
    idx = endpoint.find("?")
    if idx >= 0:
        for item in endpoint[idx + 1:].split("&"):
            k, v = item.split("=")
            params[k] = v

        endpoint = endpoint[0:idx]

    if scheme and 'scheme' not in params:
        params['scheme'] = scheme

    params["curr_project"] = project
    params_str = ""
    if params:
        params_str = "&".join(["%s=%s" % (k, v) for k, v in params.items()])
        params_str = "?" + params_str

    return "%s:%s@%s%s" % (ak, sk, endpoint, params_str)


def get_datasource():
    driver = get_driver()
    if driver == "mysql":
        dsn = get_mysql_dsn()
    elif driver == "hive":
        dsn = get_hive_dsn()
    elif driver == "maxcompute":
        dsn = get_maxcompute_dsn()
    else:
        raise ValueError("unsupported driver %s" % driver)

    return driver + "://" + dsn


SINGLETON_DB_CONNECTION = None


def get_singleton_db_connection():
    global SINGLETON_DB_CONNECTION
    if SINGLETON_DB_CONNECTION is None:
        SINGLETON_DB_CONNECTION = db.connect_with_data_source(get_datasource())

    return SINGLETON_DB_CONNECTION
