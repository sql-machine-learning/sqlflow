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

import numpy as np
from odps import ODPS, tunnel


# MaxCompute(odps) does not provide dbapi
# Here we use the sdk to operate the database
class MaxCompute:
    @staticmethod
    def connect(database, user, password, host, auth=""):
        return ODPS(user, password, project=database, endpoint=host)

    @staticmethod
    def selected_columns_and_types(conn, select):
        compress = tunnel.CompressOption.CompressAlgorithm.ODPS_ZLIB
        inst = conn.execute_sql(select)
        if not inst.is_successful():
            raise ValueError("execute {} error".format(select))

        with inst.open_reader(tunnel=True, compress_option=compress) as r:
            columns = r._schema.columns
            return [(col.name, col.type) for col in columns]

    @staticmethod
    def db_generator(conn, statement, label_meta, fetch_size):
        def reader():
            compress = tunnel.CompressOption.CompressAlgorithm.ODPS_ZLIB
            inst = conn.execute_sql(statement)
            if not inst.is_successful():
                return

            r = inst.open_reader(tunnel=True, compress_option=compress)
            field_names = None if r._schema.columns is None \
                else [col.name for col in r._schema.columns]
            if label_meta:
                try:
                    label_idx = field_names.index(label_meta["feature_name"])
                except ValueError:
                    # NOTE(typhoonzero): For clustering model, label_column_name may not in field_names when predicting.
                    label_idx = None
            else:
                label_idx = None

            i = 0
            while i < r.count:
                expected = r.count - i if r.count - i < fetch_size else fetch_size
                for row in [[v[1] for v in rec] for rec in r[i:i + expected]]:
                    # NOTE: If there is no label clause in the extended SQL, the default label value would
                    # be -1, the Model implementation can determine use it or not.
                    label = row[label_idx] if label_idx is not None else None
                    if label_meta and label_meta["delimiter"] != "":
                        if label_meta["dtype"] == "float32":
                            label = np.fromstring(label,
                                                  dtype=float,
                                                  sep=label_meta["delimiter"])
                        elif label_meta["dtype"] == "int64":
                            label = np.fromstring(label,
                                                  dtype=int,
                                                  sep=label_meta["delimiter"])

                    if label_idx is None:
                        yield list(row), None
                    else:
                        yield list(row), label
                i += expected

        return reader
