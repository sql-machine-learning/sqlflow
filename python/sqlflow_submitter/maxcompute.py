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
    def connect(database, user, password, host, auth=""):
        return ODPS(user, password, project=database, endpoint=host)

    @staticmethod
    def db_generator(conn, statement, feature_column_names, label_column_name,
                     feature_specs, fetch_size):
        def read_feature(raw_val, feature_spec):
            if feature_spec["is_sparse"]:
                indices = np.fromstring(raw_val,
                                        dtype=int,
                                        sep=feature_spec["delimiter"])
                indices = indices.reshape(indices.size, 1)
                values = np.ones([indices.size], dtype=np.int32)
                dense_shape = np.array(feature_specs[name]["shape"],
                                       dtype=np.int64)
                return (indices, values, dense_shape)
            else:
                # Dense string vector
                if feature_spec["delimiter"] != "":
                    return np.fromstring(raw_val,
                                         dtype=int,
                                         sep=feature_spec["delimiter"])
                else:
                    return (raw_val, )

        def reader():
            compress = tunnel.CompressOption.CompressAlgorithm.ODPS_ZLIB
            inst = conn.execute_sql(statement)
            if not inst.is_successful():
                return

            r = inst.open_reader(tunnel=True, compress_option=compress)
            field_names = None if r._schema.columns is None \
                else [col.name for col in r._schema.columns]
            if label_column_name:
                label_idx = field_names.index(label_column_name)
            else:
                label_idx = None

            i = 0
            while i < r.count:
                expected = r.count - i if r.count - i < fetch_size else fetch_size
                for row in [[v[1] for v in rec] for rec in r[i:i + expected]]:
                    label = row[label_idx] if label_idx is not None else None
                    features = []
                    for name in feature_column_names:
                        feature = read_feature(row[field_names.index(name)],
                                               feature_specs[name])
                        features.append(feature)
                    yield tuple(features), label
                i += expected

        return reader
