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

import tensorflow as tf
from MySQLdb import connect as mysql_connect


def connect(user, passwd, host, port):
    """connect is a convenient shortcut to mysql.connector.connect. Also,
    it makes it reasonable to import mysql.connector in this file, so
    to make it self-complete as a template.
    Args:
        user: Specifies the MySQL user name.
        passwd : Specify the MySQL password.
        host : The host name or IP address.
        port : Specifies the port number that attempts to connect
            to the MySQL server.

    """
    return mysql_connect(user=user, passwd=passwd, host=host, port=port)


def load(db, slct, label, features):
    """load returns the training features and the labels. The features is
a dict from field names to data columns, and the label is a dict from
the label field name to the label data column.

    Args:
        db: returned from MySQLdb.connect()
        slct (str): SQL SELECT statement
        label (str): the label field name as a string.
        features (list of str or None): feature field names.

    Returns:
        (features, label): maps from feature/label field names to columns.

    """
    cursor = db.cursor()
    cursor.execute(slct)
    f = [i[0] for i in cursor.description]  # get field names.
    c = list(zip(*cursor.fetchall()))  # transpose rows into columns.
    d = dict(zip(f, c))  # dict from field names to columns.
    label = d.pop(label)
    if features is not None:
        d = dict((k, d[k]) for k in features)
    return d, label


def feature_columns(features):
    """feature_columns returns a list of tf.feature_column.

    Note: currently, we are using a quick-and-hacky implementation
    that assumes all fields are numeric_columns, and not supporting
    feature derivations like tf.feature_column.cross_column.

    Args:
        features: returned from load

    Returns:
        list of tf.feature_column

    """
    return [tf.feature_column.numeric_column(key=k) for k in features.keys()]
