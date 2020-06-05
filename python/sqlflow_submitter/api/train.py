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

from sqlflow_submitter import db, tensorflow
from sqlflow_submitter.api import API_DB_CONN_CONF
from sqlflow_submitter.api.field_types import mysql_field_types


def train(sql, model, into, label, attrs={}, columns=[], validation_select=""):
    model_type = "tf"
    if model.startswith("xgboost"):
        model_type = "xgboost"

    # derive feature columns from sql statement and columns settings.
    conn = db.connect_with_data_source(API_DB_CONN_CONF["conn_str"])
    cur = conn.cursor()
    cur.execute(sql)
    field_names = []
    field_types = []
    for desc in cur.description:
        field_names.append(desc[0])
        field_types.append(mysql_field_types[desc[1]])
        # get field types
    print(field_names)
    print(field_types)
    if label not in field_names:
        raise ValueError("label (%s) is not appeared in your selected fields" %
                         label)

    if model_type == "tf":
        tensorflow.train.train(API_DB_CONN_CONF["conn_str"], model, sql,
                               validation_select)
    else:
        raise ValueError("not supported mode type: %s" % model)
