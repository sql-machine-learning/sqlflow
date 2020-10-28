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

import contextlib
import random
import string

import six
from runtime import db
from runtime.dbapi.maxcompute import MaxComputeConnection
from runtime.diagnostics import SQLFlowDiagnostic

LIFECYCLE_ON_TMP_TABLE = 7


def get_project(datasource):
    """Get the project info from given datasource

    Args:
        datasource: The odps url to extract project
    """
    _, _, _, project = MaxComputeConnection.get_uri_parts(datasource)
    return project


def create_train_and_eval_tmp_table(train_select, valid_select, datasource):
    train_table = create_tmp_table_from_select(train_select, datasource)
    valid_table = create_tmp_table_from_select(valid_select, datasource)
    return train_table, valid_table


def create_tmp_table_from_select(select, datasource):
    """Create temp table for given select query

    Args:
        select: string, the selection statement
        datasource: string, the datasource to connect
    """
    if not select:
        return None
    project = get_project(datasource)
    tmp_tb_name = gen_rand_string()
    create_sql = "CREATE TABLE %s LIFECYCLE %s AS %s" % (
        tmp_tb_name, LIFECYCLE_ON_TMP_TABLE, select)
    # (NOTE: lhw) maxcompute conn doesn't support close
    # we should unify db interface
    with db.connect_with_data_source(datasource) as conn:
        if not conn.execute(create_sql):
            raise SQLFlowDiagnostic("Can't create tmp table for %s" % select)
        return "%s.%s" % (project, tmp_tb_name)


def drop_tables(tables, datasource):
    """Drop given tables in datasource"""
    with db.connect_with_data_source(datasource) as conn:
        try:
            for table in tables:
                if table != "":
                    drop_sql = "DROP TABLE IF EXISTS %s" % table
                    conn.execute(drop_sql)
        except:  # noqa: E722
            # odps will clear table itself, so even fail here, we do
            # not need to raise error
            print("Encounter error on drop tmp table")


def gen_rand_string(slen=16):
    """generate random string with given len

    Args:
        slen: int, the length of the output string

    Returns:
        A random string with slen length
    """
    first_char = random.sample(string.ascii_letters, 1)
    rest_char = random.sample(string.ascii_letters + string.digits, slen - 1)
    return ''.join(first_char + rest_char)


@contextlib.contextmanager
def create_tmp_tables_guard(selects, datasource):
    if isinstance(selects, six.string_types):
        tables = create_tmp_table_from_select(selects, datasource)
        drop_table_list = [tables]
    elif isinstance(selects, (list, tuple)):
        tables = [create_tmp_table_from_select(s, datasource) for s in selects]
        drop_table_list = tables
    else:
        raise ValueError("not supported types {}".format(type(selects)))

    try:
        yield tables
    finally:
        drop_tables(drop_table_list, datasource)
