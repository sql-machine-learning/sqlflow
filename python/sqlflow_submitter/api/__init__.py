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

# See https://stackoverflow.com/a/3400584 for why should use a dict
API_DB_CONN_CONF = {}


def init(db_conn_str):
    global API_DB_CONN_CONF
    if not db_conn_str.startswith("mysql://"):
        raise ValueError("only support mysql currently")

    API_DB_CONN_CONF["conn_str"] = db_conn_str
    API_DB_CONN_CONF["driver"] = "mysql"
