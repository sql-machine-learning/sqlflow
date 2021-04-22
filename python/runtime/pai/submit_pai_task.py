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

import subprocess

import six

from runtime.dbapi.maxcompute import MaxComputeConnection
from runtime.diagnostics import SQLFlowDiagnostic


def run_command_and_log(cmd):
    p = subprocess.Popen(cmd,
                         bufsize=0,
                         stdout=subprocess.PIPE,
                         stderr=subprocess.STDOUT,
                         shell=False)
    for line in p.stdout:
        if six.PY3 and isinstance(line, bytes):
            line = line.decode('utf-8')

        if line is not None:
            print(line)

    p.communicate()
    return p.returncode


def submit_pai_task(pai_cmd, datasource):
    """Submit given cmd to PAI which manipulate datasource

    Args:
        pai_cmd: The command to submit
        datasource: The datasource this cmd will manipulate
    """
    user, passwd, address, project = MaxComputeConnection.get_uri_parts(
        datasource)
    cmd = [
        "odpscmd", "--instance-priority", "9", "-u", user, "-p", passwd,
        "--project", project, "--endpoint", address, "-e", pai_cmd
    ]
    exitcode = run_command_and_log(cmd)
    if exitcode != 0:
        raise SQLFlowDiagnostic("Execute odps cmd fail: cmd is %s" %
                                " ".join(cmd))
