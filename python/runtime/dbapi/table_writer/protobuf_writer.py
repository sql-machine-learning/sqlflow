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

from google.protobuf import text_format, wrappers_pb2
from runtime.dbapi.connection import ResultSet
# NOTE(sneaxiy): importing sqlflow_pb2 consumes about
# 0.24s. Do not know how to shorten the import time.
from runtime.dbapi.table_writer import sqlflow_pb2


class ProtobufWriter(object):
    def __init__(self, result_set, header=None):
        head = sqlflow_pb2.Head()
        if header is None:
            assert isinstance(result_set, ResultSet)
            column_info = result_set.raw_column_info()
            for field_name, _ in column_info:
                head.column_names.append(field_name)
        else:
            for field_name in header:
                head.column_names.append(field_name)

        self.all_responses = []
        self.all_responses.append(sqlflow_pb2.Response(head=head))
        for row in result_set:
            pb_row = sqlflow_pb2.Row()
            for col in row:
                any_msg = self.pod_to_pb_any(col)
                any = pb_row.data.add()
                any.Pack(any_msg)
            self.all_responses.append(sqlflow_pb2.Response(row=pb_row))

    @staticmethod
    def pod_to_pb_any(value):
        if value is None:
            v = sqlflow_pb2.Row.Null()
        elif isinstance(value, bool):
            v = wrappers_pb2.BoolValue(value=value)
        elif isinstance(value, int):
            v = wrappers_pb2.Int64Value(value=value)
        elif isinstance(value, float):
            v = wrappers_pb2.FloatValue(value=value)
        elif isinstance(value, str):
            v = wrappers_pb2.StringValue(value=value)
        else:
            raise ValueError("not supported cell data type: %s" % type(value))
        return v

    def dump_strings(self):
        lines = []
        for resp in self.all_responses:
            lines.append(text_format.MessageToString(resp, as_one_line=True))
        return lines
