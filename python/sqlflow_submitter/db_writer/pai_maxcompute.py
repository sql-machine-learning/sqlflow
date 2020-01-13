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

import tensorflow as tf
from odps import ODPS, tunnel

from .base import BufferedDBWriter


class PAIMaxComputeDBWriter(BufferedDBWriter):
    def __init__(self, table_name, table_schema, buff_size):
        super(PAIMaxComputeDBWriter, self).__init__(None, table_name,
                                                    table_schema, buff_size)
        table_name_parts = table_name.split(".")
        assert len(table_name_parts) == 2
        table_name_formated = "odps://%s/tables/%s" % (table_name_parts[0],
                                                       table_name_parts[1])
        self.writer = tf.python_io.TableWriter(table_name_formated, slice_id=0)
        self.writer_indices = range(len(table_schema))

    def flush(self):
        self.writer.write(self.rows, self.writer_indices)
        self.rows = []

    def close(self):
        if len(self.rows) > 0:
            self.flush()
        self.writer.close()
