# Copyright 2019 The SQLFlow Authors. All rights reserved.
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

import io
import os
import pickle
import tarfile

import odps
import tensorflow as tf
from sqlflow_submitter import db
from tensorflow.python.platform import gfile


def save(oss_model_dir, *meta):
    '''
    Save model descriptions like the training SQL statements to OSS directory.
    Data are saved using pickle.
    Args:
        oss_model_dir: OSS URI that the model will be saved to.
        *meta: python objects to be saved.
    Return:
        None
    '''
    uri_parts = oss_model_dir.split("?")
    if len(uri_parts) != 2:
        raise ValueError("error oss_model_dir: ", oss_model_dir)
    oss_path = "/".join([uri_parts[0].rstrip("/"), "sqlflow_model_desc"])
    writer = gfile.GFile(oss_path, mode='w')
    pickle.dump(list(meta), writer)
    writer.flush()
    writer.close()


def load(oss_model_dir):
    '''
    Load and restore a directory and metadata that are saved by `model.save`
    from a MaxCompute table
    Args:
        oss_model_dir: OSS URI that the model will be saved to.
    Return:
        A list contains the saved python objects
    '''
    uri_parts = oss_model_dir.split("?")
    if len(uri_parts) != 2:
        raise ValueError("error oss_model_dir: ", oss_model_dir)
    oss_path = "/".join([uri_parts[0].rstrip("/"), "sqlflow_model_desc"])

    reader = gfile.GFile(oss_path, mode='r')
    return pickle.load(reader)
