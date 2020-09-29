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


# NOTE(sneaxiy): do not import the XxxConnection object outside the
# following method. It is because those imports are quite slow (about 1-2s),
# making that the normal SQL statement runs very slow.
def get_connection_object(driver):
    if driver == "mysql":
        from runtime.dbapi.mysql import MySQLConnection
        return MySQLConnection
    elif driver == "hive":
        from runtime.dbapi.hive import HiveConnection
        return HiveConnection
    elif driver == "maxcompute":
        from runtime.dbapi.maxcompute import MaxComputeConnection
        return MaxComputeConnection
    elif driver == "paiio":
        from runtime.dbapi.paiio import PaiIOConnection
        return PaiIOConnection
    else:
        raise ValueError("unsupported driver type %s" % driver)


def connect(uri):
    """Connect to given uri

    Params:
      uri: a valid URI string

    Returns:
      A Connection object

    Raises:
      ValueError if the uri is not valid or can't find given driver
    """
    parts = uri.split("://")
    if len(parts) < 2:
        raise ValueError("Input should be a valid uri.", uri)
    return get_connection_object(parts[0])(uri)
