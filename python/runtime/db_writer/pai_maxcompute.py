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

from .base import BufferedDBWriter


class PAIMaxComputeDBWriter(BufferedDBWriter):
    def __init__(self, table_name, table_schema, buff_size, slice_id=0):
        super(PAIMaxComputeDBWriter, self).__init__(None, table_name,
                                                    table_schema, buff_size)
        table_name_parts = table_name.split(".")
        assert len(table_name_parts) == 2
        table_name_formatted = "odps://%s/tables/%s" % (table_name_parts[0],
                                                        table_name_parts[1])
        try:
            from paiio import TableWriter
        except Exception:
            from paiio.python_io import TableWriter
        self.writer = TableWriter(table_name_formatted, slice_id)
        self.writer_indices = list(range(len(table_schema)))

    def flush(self):
        self.writer.write(self.rows, self.writer_indices)
        self.rows = []

    def close(self):
        if len(self.rows) > 0:
            self.flush()
        self.writer.close()
