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

import sys
import time

from runtime.dbapi.pyalisa.client import AlisaTaksStatus, Client

# waiting task completed
WAIT_INTEVERAL_SEC = 2
# read results while a task completed
READ_RESULTS_BATCH = 20


class Task(object):  # noqa: R0205
    """Task encapsulates operations to submit the alisa task.

    Args:
        config(Config): the config for building the task
    """
    def __init__(self, config):
        self.config = config
        self.cli = Client(config)

    def exec_sql(self, code, output=sys.stdout, resultful=False):
        """submit the sql statements to alisa server, write the logs to output

        Args:
            code: sql statements
            resultful: has result
            output: like sys.stdout
        """
        task_id, status = self.cli.create_sql_task(code)
        return self._tracking(task_id, status, output, resultful)

    def exec_pyodps(self, code, args, output=sys.stdout):
        """submit the python code to alisa server, write the logs to output

        Args:
            code: python code
            args: args for python code
            output: such as sys.stdout
        """
        task_id, status = self.cli.create_pyodps_task(code, args)
        return self._tracking(task_id, status, output, False)

    def _tracking(self, task_id, status, output, resultful):
        return self._tracking_with_log(
            task_id, status, output,
            resultful) if self.config.verbose else self._tracking_quietly(
                task_id, status, resultful)

    def _tracking_with_log(self, task_id, status, output, resultful):
        log_idx = 0
        while not self.cli.completed(status):
            if status in (AlisaTaksStatus.ALISA_TASK_WAITING,
                          AlisaTaksStatus.ALISA_TASK_ALLOCATE):
                output.write('waiting for resources')
            elif status == AlisaTaksStatus.ALISA_TASK_RUNNING and log_idx >= 0:
                self.cli.read_logs(task_id, log_idx, output)
            time.sleep(WAIT_INTEVERAL_SEC)
            status = self.cli.get_status(task_id)

        if status == AlisaTaksStatus.ALISA_TASK_EXPIRED:
            output.write('timeout while waiting for resources')
        else:
            # assert log_idx>=0
            self.cli.read_logs(task_id, log_idx, output)
            # assert log_idex<0
            if status == AlisaTaksStatus.ALISA_TASK_COMPLETED:
                return self.cli.get_results(
                    task_id, READ_RESULTS_BATCH) if resultful else []
        raise Exception('task={}, invalid status={}'.format(task_id, status))

    def _tracking_quietly(self, task_id, status, resultful):
        while not self.cli.completed(status):
            time.sleep(WAIT_INTEVERAL_SEC)
            status = self.cli.get_status(task_id)

        if status != AlisaTaksStatus.ALISA_TASK_COMPLETED:
            raise Exception(
                'task({}) status is {} which means incompleted.'.format(
                    task_id, status))

        if resultful:
            return self.cli.get_results(task_id, READ_RESULTS_BATCH)
        return []
