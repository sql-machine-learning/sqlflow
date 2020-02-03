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

import io
import os
import pickle
import tarfile

import odps
import tensorflow as tf
from sqlflow_submitter import db
from tensorflow.python.platform import gfile


def get_oss_path_from_uri(oss_model_dir, file_name):
    uri_parts = oss_model_dir.split("?")
    if len(uri_parts) != 2:
        raise ValueError("error oss_model_dir: ", oss_model_dir)
    oss_path = "/".join([uri_parts[0].rstrip("/"), file_name])
    return oss_path


def save_file(oss_model_dir, file_name):
    '''
    Save the local file to OSS direcotory using GFile.
    '''
    print("creating oss dirs: %s" % oss_model_dir)
    oss_path = get_oss_path_from_uri(oss_model_dir, file_name)
    oss_dir = oss_model_dir.split("?")[0]
    print("creating oss dirs: %s" % oss_dir)
    gfile.MakeDirs(oss_dir)
    fn = open(file_name, "r")
    writer = gfile.GFile(oss_path, mode='w')
    while True:
        buf = fn.read(4096)
        if not buf:
            break
        writer.write(buf)
    writer.flush()
    writer.close()
    fn.close()


def load_file(oss_model_dir, file_name):
    '''
    Load file from OSS to local directory.
    '''
    oss_path = get_oss_path_from_uri(oss_model_dir, file_name)
    fn = open(file_name, "w")
    reader = gfile.GFile(oss_path, mode="r")
    while True:
        buf = reader.read(4096)
        if not buf:
            break
        fn.write(buf)
    reader.close()
    fn.close()


def save_metas(oss_model_dir, num_workers, file_name, *meta):
    '''
    Save model descriptions like the training SQL statements to OSS directory.
    Data are saved using pickle.
    Args:
        oss_model_dir: OSS URI that the model will be saved to.
        *meta: python objects to be saved.
    Return:
        None
    '''
    if num_workers > 1:
        FLAGS = tf.app.flags.FLAGS
        if FLAGS.task_index != 0:
            print("skip saving model desc on workers other than worker 0")
            return
    oss_path = get_oss_path_from_uri(oss_model_dir, file_name)
    writer = gfile.GFile(oss_path, mode='w')
    pickle.dump(list(meta), writer)
    writer.flush()
    writer.close()
    # write a file "file_name_estimator" to store the estimator name, so we
    # can determine if the estimator is BoostedTrees* when explaining the model.
    oss_path = get_oss_path_from_uri(oss_model_dir,
                                     "_".join([file_name, "estimator"]))
    writer = gfile.GFile(oss_path, mode='w')
    writer.write(meta[0])
    writer.flush()
    writer.close()


def load_metas(oss_model_dir, file_name):
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
    oss_path = "/".join([uri_parts[0].rstrip("/"), file_name])

    reader = gfile.GFile(oss_path, mode='r')
    return pickle.load(reader)
