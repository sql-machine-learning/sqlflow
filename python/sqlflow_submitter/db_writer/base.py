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

import sys

if sys.version > '3':
    from abc import ABC, abstractmethod

    class BufferedDBWriter(ABC):
        def __init__(self, conn, table_name, table_schema, buff_size=100):
            self.conn = conn
            self.table_name = table_name
            self.table_schema = table_schema
            self.buff_size = buff_size
            self.rows = []

        @abstractmethod
        def flush(self):
            return

        def write(self, value):
            self.rows.append(value)
            if len(self.rows) > self.buff_size:
                self.flush()

        def close(self):
            if len(self.rows) > 0:
                self.flush()
else:
    from abc import ABCMeta, abstractmethod

    class BufferedDBWriter():
        __metaclass__ = ABCMeta

        def __init__(self, conn, table_name, table_schema, buff_size=100):
            self.conn = conn
            self.table_name = table_name
            self.table_schema = table_schema
            self.buff_size = buff_size
            self.rows = []

        @abstractmethod
        def flush(self):
            return

        def write(self, value):
            self.rows.append(value)
            if len(self.rows) > self.buff_size:
                self.flush()

        def close(self):
            if len(self.rows) > 0:
                self.flush()
