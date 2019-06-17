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

import numpy as np
from odps import ODPS, tunnel

# MaxCompute(odps) does not provide dbapi
# Here we use the sdk to operate the database
class MaxCompute:
    @staticmethod
    def connect(database, user, password, host):
        return ODPS(user, password, project=database, endpoint=host)
    
    @staticmethod
    def db_generator(conn, statement, feature_column_names,
            label_column_name, column_name_to_type, fetch_size):
        def reader():
            compress = tunnel.CompressOption.CompressAlgorithm.ODPS_ZLIB
            inst = conn.execute_sql(statement)
            if not inst.is_successful():
                return None
    
            r = inst.open_reader(tunnel=True, compress_option=compress)
            field_names = None if r._schema.columns is None \
                    else [col.name for col in r._schema.columns]
            label_idx = field_names.index(label_column_name)
    
            i = 0
            while i < r.count:
                expected = r.count-i if r.count-i < fetch_size else fetch_size
                for row in [[v[1] for v in rec] for rec in r[i: i+expected]]:
                    label = row[label_idx]
                    features = dict()
                    for name in feature_column_names:
                        if column_name_to_type[name] == "categorical_column_with_identity":
                            cell = np.fromstring(row[field_names.index(name)], dtype=int, sep=",")
                        else:
                            cell = row[field_names.index(name)]
                        features[name] = cell
                    yield (features, [label])
                i += expected
        return reader
    
    @staticmethod
    def insert_values(conn, table, values):
        compress = tunnel.CompressOption.CompressAlgorithm.ODPS_ZLIB
        conn.write_table(table, values, compress_option=compress)
