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

import json

from google.protobuf import text_format
from sqlflow_submitter import db

from .. import features
from ..proto import ir_pb2
from . import default


def submit(statement, datasource, feature_specs, label_spec):
    with open("specs.json", "w") as fpkl, open("stmt.pb", "w") as fpb:
        json.dump((datasource, feature_specs, label_spec), fpkl)
        fpb.write(str(statement))


def execute(program):
    conn = db.connect_with_data_source(program.datasource)
    for stmt in program.statements:
        if stmt.type == ir_pb2.Statement.QUERY:
            default.query(conn, stmt.select)
            continue
        submit(stmt, program.datasource,
               *features.get_feature_specs(conn, stmt.select, stmt.label))


def entry():
    stmt = ir_pb2.Statement()
    text_format.Parse(open("stmt.pb").read(), stmt)
    if stmt.type == ir_pb2.Statement.TRAIN:
        default.train(stmt, *json.load(open("specs.json")))


if __name__ == '__main__':
    entry()
