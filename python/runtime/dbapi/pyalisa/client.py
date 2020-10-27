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
# limitations under the License


class Client(object):
    def __init__(self, access_id):
        pass

    def create_task(self, params):
        pass
        # params["CustomerId"] = ali.With["CustomerId"]
        # params["UniqueKey"] = fmt.Sprintf("%d", time.Now().UnixNano())
        # params["ExecTarget"] = ali.Env["ALISA_TASK_EXEC_TARGET"]

    def create_sql_task(self, code):
        pass

    def create_pyodps_task(self, code, args):
        pass
