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

from runtime.dbapi.maxcompute import MaxComputeConnection
from runtime.diagnostics import SQLFlowDiagnostic


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
    print(" ".join(cmd))
    if subprocess.call(cmd) != 0:
        raise SQLFlowDiagnostic("Execute odps cmd fail: cmd is %s" %
                                " ".join(cmd))
