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

import oss2
import tensorflow as tf
from runtime import db
from runtime.pai.oss import get_bucket
from runtime.tensorflow import is_tf_estimator

# NOTE(typhoonzero): hard code bucket name "sqlflow-models" as the bucket to save models trained.
SQLFLOW_MODELS_BUCKET = "sqlflow-models"

# ModelTypeTF is the mode type that trained by PAI Tensorflow.
MODEL_TYPE_TF = 1
# ModelTypeXGBoost is the model type that use PAI Tensorflow to train XGBoost models.
MODEL_TYPE_XGB = 2
# ModelTypePAIML is the model type that trained by PAI machine learning algorithm toolkit
MODEL_TYPE_PAIML = 3


def get_models_bucket():
    return get_bucket(SQLFLOW_MODELS_BUCKET)


def remove_bucket_prefix(oss_uri):
    return oss_uri.replace("oss://%s/" % SQLFLOW_MODELS_BUCKET, "")


def get_oss_path_from_uri(oss_model_dir, file_name):
    # oss_model_dir is of format: oss://bucket/path/to/dir/
    assert (oss_model_dir.startswith("oss://"))
    oss_file_path = "/".join([oss_model_dir.rstrip("/"), file_name])
    return oss_file_path


def mkdir(bucket, oss_dir):
    assert (oss_dir.startswith("oss://"))
    if not oss_dir.endswith("/"):
        oss_dir = oss_dir + "/"
    path = remove_bucket_prefix(oss_dir)
    has_dir = True
    try:
        meta = bucket.get_object_meta(path)
    except oss2.exceptions.NoSuchKey:
        has_dir = False
    except Exception as e:
        raise e

    if not has_dir:
        bucket.put_object(path, "")


def save_dir(oss_model_dir, local_dir):
    '''
    Recursively upload local_dir under oss_model_dir
    '''
    bucket = get_models_bucket()
    for (root, dirs, files) in os.walk(local_dir, topdown=True):
        dst_dir = "/".join([oss_model_dir.rstrip("/"), root])
        mkdir(bucket, dst_dir)
        for file_name in files:
            curr_file_path = os.path.join(root, file_name)
            remote_file_path = "/".join([dst_dir.rstrip("/"), file_name])
            remote_file_path = remove_bucket_prefix(remote_file_path)
            bucket.put_object_from_file(remote_file_path, curr_file_path)


def load_dir(oss_model_dir):
    bucket = get_models_bucket()
    path = remove_bucket_prefix(oss_model_dir)
    prefix = "/".join(path.split("/")[:-1]) + "/"
    for obj in oss2.ObjectIterator(bucket, prefix=path):
        # remove prefix when writing to local, e.g.
        # remote: path/to/my/dir/
        # local: dir/
        if obj.key.endswith("/"):
            os.makedirs(obj.key.replace(prefix, ""))
        else:
            bucket.get_object_to_file(obj.key, obj.key.replace(prefix, ""))


def save_file(oss_model_dir, file_name):
    '''
    Save the local file (file_name is a file under current directory) to OSS directory.
    '''
    bucket = get_models_bucket()
    oss_path = get_oss_path_from_uri(oss_model_dir, file_name)
    oss_path = remove_bucket_prefix(oss_path)

    mkdir(bucket, oss_model_dir)
    bucket.put_object_from_file(oss_path, file_name)


def save_string(oss_file_path, data):
    '''
    Save a string into an oss_file_path
    '''
    bucket = get_models_bucket()
    oss_dir = "/".join(oss_file_path.split("/")[:-1])
    mkdir(bucket, oss_dir)
    oss_file_path = remove_bucket_prefix(oss_file_path)
    bucket.put_object(oss_file_path, data)


def load_file(oss_model_dir, file_name):
    '''
    Load file from OSS to local directory.
    '''
    oss_file_path = "/".join([oss_model_dir.rstrip("/"), file_name])
    oss_file_path = remove_bucket_prefix(oss_file_path)
    bucket = get_models_bucket()
    bucket.get_object_to_file(oss_file_path, file_name)


def load_string(oss_file_path):
    bucket = get_models_bucket()
    oss_file_path = remove_bucket_prefix(oss_file_path)
    data = bucket.get_object(oss_file_path).read()
    return data.decode("utf-8")


def save_metas(oss_model_dir, num_workers, file_name, *meta):
    '''
    Save model descriptions like the training SQL statements to OSS directory.
    Data are saved using pickle.
    it will report "can't pickle weakref objects" when using pickle.
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
    serialized = pickle.dumps(list(meta))
    save_string(oss_path, serialized)

    # write a file "file_name_estimator" to store the estimator name, so we
    # can determine if the estimator is BoostedTrees* when explaining the model.
    estimator_file_name = "_".join([file_name, "estimator"])
    oss_path = get_oss_path_from_uri(oss_model_dir, estimator_file_name)
    save_string(oss_path, meta[0])


def load_metas(oss_model_dir, file_name):
    '''
    Load and restore a directory and metadata that are saved by `model.save`
    from a MaxCompute table
    Args:
        oss_model_dir: OSS URI that the model will be saved to.
    Return:
        A list contains the saved python objects
    '''
    oss_path = "/".join([oss_model_dir.rstrip("/"), file_name])
    serialized = load_string(oss_path)
    return pickle.loads(serialized)


def load_oss_model(oss_model_dir, estimator):
    is_estimator = is_tf_estimator(estimator)
    # Keras single node is using h5 format to save the model, no need to deal with export model format.
    # Keras distributed mode will use estimator, so this is also needed.
    if is_estimator:
        load_file(oss_model_dir, "exported_path")
        # NOTE(typhoonzero): directory "model_save" is hardcoded in codegen/tensorflow/codegen.go
        load_dir(oss_model_dir + "/model_save")
    else:
        load_file(oss_model_dir, "model_save")


def save_oss_model(oss_model_dir, estimator_name, is_estimator,
                   feature_column_names, feature_column_names_map,
                   feature_metas, label_meta, model_params,
                   feature_columns_code, num_workers):
    # Keras single node is using h5 format to save the model, no need to deal with export model format.
    # Keras distributed mode will use estimator, so this is also needed.
    if is_estimator:
        with open("exported_path", "rb") as fn:
            saved_model_path = fn.read()
        save_dir(oss_model_dir, saved_model_path)
        save_file(oss_model_dir, "exported_path")
    else:
        if num_workers > 1:
            save_file(oss_model_dir, "exported_path")
        else:
            save_file(oss_model_dir, "model_save")

    save_metas(oss_model_dir, num_workers, "tensorflow_model_desc",
               estimator_name, feature_column_names, feature_column_names_map,
               feature_metas, label_meta, model_params, feature_columns_code)
