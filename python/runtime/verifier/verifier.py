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

import runtime.db as db


def fetch_samples(conn, query, n=1):
    query = db.limit_select(query, n)
    for rows, _ in db.db_generator(conn, query)():
        yield rows


def verify_column_name_and_type(conn, train_select, pred_select, label):
    train_schema = db.selected_columns_and_types(conn, train_select)
    pred_schema = dict(db.selected_columns_and_types(conn, pred_select))

    for name, train_type in train_schema:
        if name == label:
            continue

        pred_type = pred_schema.get(name)
        if pred_type is None:
            raise ValueError(
                "the predict statement doesn't contain column %s" % name)

        if pred_type != train_type:
            raise ValueError(
                "field %s type dismatch %s(predict) vs %s(train)" %
                (name, pred_type, train_type))
