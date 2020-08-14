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

from runtime.dbapi.hive import HiveConnection
from runtime.dbapi.maxcompute import MaxComputeConnection
from runtime.dbapi.mysql import MySQLConnection
from runtime.dbapi.paiio import PaiIOConnection

DRIVER_MAP = {
    "mysql": MySQLConnection,
    "hive": HiveConnection,
    "maxcompute": MaxComputeConnection,
    "paiio": PaiIOConnection
}


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
        raise ValueError("Input should be a valid uri.")
    if parts[0] not in DRIVER_MAP:
        raise ValueError("Can't find driver for scheme: %s" % parts[0])
    return DRIVER_MAP[parts[0]](uri)
