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

import ast
import json
import random
import string
import time
from enum import Enum

from runtime.dbapi.pyalisa.pop import Pop


class AlisaTaksStatus(Enum):
    ALISA_TASK_WAITING = 1
    ALISA_TASK_RUNNING = 2
    ALISA_TASK_COMPLETED = 3
    ALISA_TASK_ERROR = 4
    ALISA_TASK_FAILOVER = 5
    ALISA_TASK_KILLED = 6
    ALISA_TASK_RERUN = 8
    ALISA_TASK_EXPIRED = 9
    ALISA_TASK_ALISA_RERUN = 10
    ALISA_TASK_ALLOCATE = 11


# used to deal with too many logs
MAX_LOG_NUM = 2000


class Client(object):
    """Client for building kinds of tasks and submitting them to alisa gateway

    Args:
        config(Config): the Config(runtime.dbapi.pyalisa.config)
    """
    def __init__(self, config):
        self.config = config  # noqa F841

    def _base_params(self):
        # use gmtime(UTC+0) here instead of localtime
        ts = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
        nonce = "".join(random.sample(string.ascii_letters, 32))
        return {
            "Timestamp": ts,
            "AccessKeyId": self.config.pop_access_id,
            "SignatureMethod": "HMAC-SHA1",
            "SignatureVersion": "1.0",
            "SignatureNonce": nonce,
            "Format": "JSON",
            "Product": "dataworks",
            "Version": "2017-12-12",
        }

    def create_sql_task(self, code):
        """Create a SQL task and return the result

        Args:
            code(string): the SQL program to run

        Returns:
            a (taskId, statu) tuple
        """
        params = self._base_params()
        params["ExecCode"] = code
        params["PluginName"] = self.config.withs["PluginName"]
        params["Exec"] = self.config.withs["Exec"]
        return self._create_task(params)

    def create_pyodps_task(self, code, args):
        """Create a pyodps task and return the result

        Args:
            code(string): the pyodps program to run

        Returns:
            a (taskId, status) tuple
        """
        params = self._base_params()
        params["ExecCode"] = code
        params["PluginName"] = self.config.withs["PluginName4PyODPS"]
        params["Exec"] = self.config.withs["Exec4PyODPS"]
        if len(args) > 0:
            params["Args"] = args
        return self._create_task(params)

    def _create_task(self, params):
        """ Create returns a task id and it's status

        Args:
            params(dict): kinds of request params, both keys and values
                should be strings

        Returns:
            a (taskId, status) tuple
        """
        params["CustomerId"] = self.config.withs["CustomerId"]
        params["UniqueKey"] = str(time.time())
        params["ExecTarget"] = self.config.env["ALISA_TASK_EXEC_TARGET"]

        nenv = dict(self.config.env)
        # display column type, for feature derivation.
        nenv["SHOW_COLUMN_TYPE"] = "true"
        params["Envs"] = json.dumps(nenv)
        val = self._request_and_parse_response("CreateAlisaTask", params)
        return val["alisaTaskId"], val["status"]

    def get_status(self, task_id):
        """Get the status of given task

        Args:
            task_id(string): the task id returned by create_task

        Returns:
            an AlisaTaksStatus indicating the status
        """
        params = self._base_params()
        params["AlisaTaskId"] = task_id
        val = self._request_and_parse_response("GetAlisaTask", params)
        return AlisaTaksStatus(int(val["status"]))

    def completed(self, status):
        """Check whether the given status is a finish status

        Args:
            status(AlisaTaksStatus|int): the status to check

        Returns:
            True for finish status, Flase otherwise
        """
        if isinstance(status, int):
            status = AlisaTaksStatus(status)
        return status in [
            AlisaTaksStatus.ALISA_TASK_COMPLETED,
            AlisaTaksStatus.ALISA_TASK_ERROR,
            AlisaTaksStatus.ALISA_TASK_KILLED,
            AlisaTaksStatus.ALISA_TASK_RERUN,
            AlisaTaksStatus.ALISA_TASK_EXPIRED
        ]

    def read_logs(self, task_id, offset, w):
        """Read logs for given task

        Args:
            task_id(string): the task to retrival logs
            offset(int): the log offset from where to read
            w(writer): the output stream to write the log

        Returns:
            an integer: -1 if the log is read completely, or,
            a positive integer for the end of current reading
        """

        for _ in range(MAX_LOG_NUM):
            params = self._base_params()
            params["AlisaTaskId"] = task_id
            params["Offset"] = str(offset)
            log = self._request_and_parse_response("GetAlisaTaskLog", params)
            rlen = int(log["readLength"])
            if rlen == 0:
                return offset
            offset += rlen
            w.write(log["logMsg"])
            if bool(log["isEnd"]):
                return -1
        return offset

    def count_results(self, task_id):
        """Retrival the result count for given task

        Args:
            task_id(string): the task to query

        Returns:
            an integer indicating the result count
        """
        params = self._base_params()
        params["AlisaTaskId"] = task_id
        res = self._request_and_parse_response("GetAlisaTaskResultCount",
                                               params)
        return int(res)

    def get_results(self, task_id, batch):
        """Reads the task's results

        Args:
            task_id(string): the task to read
            batch(int): batch size for retrival the result

        Returns:
            a list of query results
        """
        if batch <= 0:
            raise ValueError("batch should greater than 0")
        count = self.count_results(task_id)

        columns, body = [], []
        for i in range(0, count, batch):
            params = self._base_params()
            params["AlisaTaskId"] = task_id
            params["Start"] = str(i)
            params["Limit"] = str(batch)
            val = self._request_and_parse_response("GetAlisaTaskResult",
                                                   params)
            header, rows = self._parse_alisa_value(val)
            if len(columns) == 0:
                columns = header
            body.extend(rows)
        return {"columns": columns, "body": body}

    def stop(self, task_id):
        """Stop given task.
        NOTE(weiguoz): need to be tested.

        Args:
            task_id(string): the task to stop

        Returns:
            True if the task is stopped, False otherwise
        """
        params = self._base_params()
        params["AlisaTaskId"] = task_id
        res = self._request_and_parse_response("StopAlisaTask", params)
        return bool(res)

    def _parse_alisa_value(self, val):
        """Parse 'returnValue' in alisa response
        https://github.com/sql-machine-learning/goalisa/blob/68d3aad1344c9e5c0cd35c6556e1f3f2b6ca9db7/alisa.go#L190

        Args:
            val: [{u'resultMsg': u'[["Alice","23.8","56000"]]',
            u'dataHeader': u'["name::string","age::double","salary::bigint"]'}]
        """
        jsval = ast.literal_eval(json.dumps(val))
        columns = []
        for h in json.loads(jsval['dataHeader']):
            nt = h.split("::")
            name, typ = (nt[0], nt[1]) if len(nt) == 2 else (h, "string")
            columns.append({"name": str(name), "typ": str(typ)})
        body = []
        for m in json.loads(jsval['resultMsg']):
            row = []
            for i in ast.literal_eval(json.dumps(m)):
                row.append(i)
            body.append(row)
        return columns, body

    def _request_and_parse_response(self, action, params):
        params["Action"] = action
        params["ProjectEnv"] = self.config.env["SKYNET_SYSTEM_ENV"]
        url = self.config.pop_scheme + "://" + self.config.pop_url
        code, buf = Pop.request(url, params, self.config.pop_access_secret)
        resp = json.loads(buf)
        if code != 200:
            raise RuntimeError("%s got a bad result, request=%s, response=%s" %
                               (code, params, buf))
        if resp['returnCode'] != '0':
            raise Exception("returned an error request={}, response={}".format(
                params, resp))
        return resp["returnValue"]
