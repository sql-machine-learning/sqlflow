from odps import ODPS, tunnel

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


class MaxCompute:
    @staticmethod
    def connect(database, user, password, host):
        print("project=", database)
        return ODPS(user, password, project=database, endpoint=host)

    @staticmethod
    def execute(conn, statement):
        compress = tunnel.CompressOption.CompressAlgorithm.ODPS_ZLIB
        inst = conn.execute_sql(statement)
        if not inst.is_successful():
            return None, None
        r = inst.open_reader(tunnel=True, compress_option=compress)
        field_names = [col.name for col in r._schema.columns]
        rows = [[v[1] for v in rec] for rec in r[0: r.count]]
        return field_names, list(map(list, zip(*rows))) if r.count > 0 else None

    @staticmethod
    def insert_values(conn, table, values):
        compress = tunnel.CompressOption.CompressAlgorithm.ODPS_ZLIB
        conn.write_table(table, values, compress_option=compress)

