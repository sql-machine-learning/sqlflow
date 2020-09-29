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
    INT64 = 0
    FLOAT32 = 1
    STRING = 2

    @staticmethod
    def to_string(dtype):
        if dtype == DataType.INT64:
            return "int64"

        if dtype == DataType.FLOAT32:
            return "float32"

        if dtype == DataType.STRING:
            return "string"

        raise ValueError("unsupported data type {}".format(dtype))

    @staticmethod
    def to_db_field_type(driver, dtype):
        """
        This method converts the dtype to a field type that the CREATE
        TABLE statement accepts.

        Args:
            driver (str): the DBMS driver type.
            dtype (enum): the data type. One of FLOAT32, INT64 and STRING.

        Returns:
            A field type that the CREATE TABLE statement accepts.
        """
        if dtype == DataType.INT64:
            return "BIGINT"

        if dtype == DataType.FLOAT32:
            return "DOUBLE"

        if dtype == DataType.STRING:
            if driver == "mysql":
                return "VARCHAR(255)"
            return "STRING"

        raise ValueError("unsupported data type {}".format(dtype))


# DataFormat is used in FieldDesc to represent the data format
# of a database field.
# PLAIN: a plain number or string, like 93.7 or "abc"
# CSV: in the form of "1,2,4"
# KV:  in the form of "0:3.2 1:-0.3 10:3.9"
class DataFormat(object):
    PLAIN = ""
    CSV = "csv"
    KV = "kv"


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
        vocabulary (list[str]): the vocabulary used for categorical
            feature column. Default None.
        max_id (int): the maximum id number of the field data. Used in
            CategoryIDColumn. Default 0.
    """
    def __init__(self,
                 name="",
                 dtype=DataType.INT64,
                 dtype_weight=DataType.INT64,
                 delimiter="",
                 delimiter_kv="",
                 format=DataFormat.PLAIN,
                 shape=None,
                 is_sparse=False,
                 vocabulary=None,
                 max_id=0):
        assert dtype in [DataType.INT64, DataType.FLOAT32, DataType.STRING]
        assert dtype_weight in [
            DataType.INT64, DataType.FLOAT32, DataType.STRING
        ]
        assert format in [DataFormat.CSV, DataFormat.KV, DataFormat.PLAIN]

        self.name = name
        self.dtype = dtype
        self.dtype_weight = dtype_weight
        self.delimiter = delimiter
        self.delimiter_kv = delimiter_kv
        self.format = format
        self.shape = shape
        self.is_sparse = is_sparse
        if vocabulary is not None:
            vocabulary = set(list(vocabulary))
        self.vocabulary = vocabulary
        self.max_id = max_id

    def to_dict(self, dtype_to_string=False):
        """
        Convert the FieldDesc object to a Python dict.

        Returns:
            A Python dict.
        """
        vocab = None
        if self.vocabulary is not None:
            vocab = list(self.vocabulary)
            vocab.sort()

        if dtype_to_string:
            dtype = DataType.to_string(self.dtype)
            dtype_weight = DataType.to_string(self.dtype_weight)
        else:
            dtype = self.dtype
            dtype_weight = self.dtype_weight

        return {
            "name": self.name,
            # FIXME(typhoonzero): this line is used to be compatible to
            # current code, remove it after the refactor.
            "feature_name": self.name,
            "dtype": dtype,
            "dtype_weight": dtype_weight,
            "delimiter": self.delimiter,
            "delimiter_kv": self.delimiter_kv,
            "format": self.format,
            "shape": self.shape,
            "is_sparse": self.is_sparse,
            "vocabulary": vocab,
            "max_id": self.max_id,
        }

    @classmethod
    def from_dict(cls, d):
        """
        Create a FieldDesc object from a Python dict.

        Returns:
            A FieldDesc object.
        """
        return FieldDesc(name=d["name"],
                         dtype=d["dtype"],
                         dtype_weight=d["dtype_weight"],
                         delimiter=d["delimiter"],
                         delimiter_kv=d["delimiter_kv"],
                         format=d["format"],
                         shape=d["shape"],
                         is_sparse=d["is_sparse"],
                         vocabulary=d["vocabulary"],
                         max_id=d["max_id"])

    def to_json(self):
        """
        Convert the FieldDesc object to a json string.

        Returns:
            A string which represents the json value of the FieldDesc object.
        """
        return json.dumps(self.to_dict())

    @classmethod
    def from_json(cls, s):
        """
        Create a FieldDesc object from a json string.

        Args:
            s (str): the JSON string.

        Returns:
            A FieldDesc object.
        """
        return cls.from_dict(**json.loads(s))
