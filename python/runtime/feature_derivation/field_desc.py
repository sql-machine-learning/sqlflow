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

__all__ = [
    'DataType',
    'DataFormat',
    'FieldDesc',
]


# DataType is used in FieldDesc to represent the data type of
# a database field.
class DataType(object):
    INT = 0
    FLOAT = 1
    STRING = 2


# DataFormat is used in FieldDesc to represent the data format
# of a database field.
# PLAIN: a plain number or string, like 93.7 or "abc"
# CSV: in the form of "1,2,4"
# KV:  in the form of "0:3.2 1:-0.3 10:3.9"
class DataFormat(object):
    PLAIN = 0
    CSV = 1
    KV = 2


class FieldDesc(object):
    """
    FieldDesc describes a field used as the input to a feature column.

    Args:
        name (str): the field name. Default "".
        dtype (enum): the data type of the field. It must be one of INT,
            FLOAT and STRING. Default INT.
        delimiter (str): the delimiter of the field data. Default "".
        format (enum): the data format of the field data. It must be one of
            PLAIN, CSV, KV. Default PLAIN.
        shape (list[int]): the shape of the field data. Default None.
        is_sparse (bool): whether the field data is sparse. Default False.
        vocabulary (list[str]): the vocabulary used for categorical feature column. Default None.
        max_id (int): the maximum id number of the field data. Used in CategoryIDColumn. Default 0.
    """
    def __init__(self,
                 name="",
                 dtype=DataType.INT,
                 delimiter="",
                 format=DataFormat.PLAIN,
                 shape=None,
                 is_sparse=False,
                 vocabulary=None,
                 max_id=0):
        assert dtype in [DataType.INT, DataType.FLOAT, DataType.STRING]
        assert format in [DataFormat.CSV, DataFormat.KV, DataFormat.PLAIN]

        self.name = name
        self.dtype = dtype
        self.delimiter = delimiter
        self.format = format
        self.shape = shape
        self.is_sparse = is_sparse
        self.vocabulary = vocabulary
        self.max_id = max_id

    def to_json(self):
        """
        Convert the FieldDesc object to a json string.

        Returns:
            A string which represents the json value of the FieldDesc object.
        """
        return json.dumps({
            "name": self.name,
            "dtype": self.dtype,
            "delimiter": self.delimiter,
            "format": self.format,
            "shape": self.shape,
            "is_sparse": self.is_sparse,
            "vocabulary": self.vocabulary,
            "max_id": self.max_id,
        })
