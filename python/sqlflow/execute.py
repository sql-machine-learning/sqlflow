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

import os
import sys

from google.protobuf import text_format

from . import ir_pb2


def get_platform_module(name):
    name = name or "default"
    import importlib
    return importlib.import_module(f"sqlflow.platform.{name}")


def step(program):
    platform = get_platform_module(os.getenv("SQLFLOW_submitter"))
    platform.execute(program)


if __name__ == "__main__":
    if len(sys.argv) == 1:
        sys.argv += ["execute"]
    assert sys.argv[1] in ["step", "execute"]
    program = ir_pb2.Program()
    text_format.Parse(sys.stdin.read(), program)
    eval(sys.argv[1])(program)
