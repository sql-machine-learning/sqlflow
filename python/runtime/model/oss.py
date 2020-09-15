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

import os
import pickle
import sys

import oss2
import tensorflow as tf
from runtime.diagnostics import SQLFlowDiagnostic
from runtime.tensorflow import is_tf_estimator

# NOTE(typhoonzero): hard code bucket name "sqlflow-models" as the bucket to
# save models trained.
SQLFLOW_MODELS_BUCKET = "sqlflow-models"


def remove_bucket_prefix(oss_uri):
    return oss_uri.replace("oss://%s/" % SQLFLOW_MODELS_BUCKET, "")


def get_models_bucket():
    return get_bucket(SQLFLOW_MODELS_BUCKET)


def get_bucket(name, ak=None, sk=None, endpoint=None):
    if ak is None:
        ak = os.getenv("SQLFLOW_OSS_AK", "")
    if sk is None:
        sk = os.getenv("SQLFLOW_OSS_SK", "")
    if endpoint is None:
        endpoint = os.getenv("SQLFLOW_OSS_MODEL_ENDPOINT", "")
    if ak == "" or sk == "":
        raise ValueError("must configure SQLFLOW_OSS_AK and SQLFLOW_OSS_SK "
                         "when submitting to PAI")
    if endpoint == "":
        raise ValueError(
            "must configure SQLFLOW_OSS_MODEL_ENDPOINT when submitting to PAI")
    auth = oss2.Auth(ak, sk)
    bucket = oss2.Bucket(auth, endpoint, name)
    return bucket


def copyfileobj(source, dest, ak, sk, endpoint, bucket_name):
    '''
    copy_file_to_oss copies alocal file (source) to an object on OSS (dest),
    overwrite if the oss object exists.
    '''
    auth = oss2.Auth(ak, sk)
    bucket = oss2.Bucket(auth, endpoint, bucket_name)
    bucket.put_object_from_file(dest, source)


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
        bucket.get_object_meta(path)
    except oss2.exceptions.NoSuchKey:
        has_dir = False
    except Exception as e:
        raise e

    if not has_dir:
        bucket.put_object(path, "")


def delete_oss_dir_recursive(bucket, directory):
    """
    Recursively delete a directory on the OSS

    Args:
        bucket: bucket on OSS
        directory (str): the directory to delete

    Returns:
        None.
    """
    if not directory.endswith("/"):
        raise SQLFlowDiagnostic("dir to delete must end with /")

    loc = bucket.list_objects(prefix=directory, delimiter="/")
    object_path_list = []
    for obj in loc.object_list:
        object_path_list.append(obj.key)

    # delete sub dir first
    if len(loc.prefix_list) > 0:
        for sub_prefix in loc.prefix_list:
            delete_oss_dir_recursive(bucket, sub_prefix)
    # empty list param will raise error
    if len(object_path_list) > 0:
        bucket.batch_delete_objects(object_path_list)


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
            try:
                os.makedirs(obj.key.replace(prefix, ""))
            except Exception as e:
                sys.stderr.write("mkdir exception: %s\n" % str(e))
        else:
            bucket.get_object_to_file(obj.key, obj.key.replace(prefix, ""))


def save_file(oss_model_dir, local_file_name, oss_file_name=None):
    """
    Save the local file (file_name is a file under current directory)
    to OSS directory.

    Args:
        oss_model_dir (str): the OSS model directory. It is in the format
            of oss://bucket/path/to/dir/.
        local_file_name (str): the local file path.
        oss_file_name (str): the OSS file path to save. If None,
            use local_file_name as oss_file_name.

    Returns:
        None.
    """
    if oss_file_name is None:
        oss_file_name = local_file_name

    bucket = get_models_bucket()
    oss_path = get_oss_path_from_uri(oss_model_dir, oss_file_name)
    oss_path = remove_bucket_prefix(oss_path)

    mkdir(bucket, oss_model_dir)
    bucket.put_object_from_file(oss_path, local_file_name)


def save_string(oss_file_path, data):
    '''
    Save a string into an oss_file_path
    '''
    bucket = get_models_bucket()
    oss_dir = "/".join(oss_file_path.split("/")[:-1])
    mkdir(bucket, oss_dir)
    oss_file_path = remove_bucket_prefix(oss_file_path)
    bucket.put_object(oss_file_path, data)


def load_file(oss_model_dir, local_file_name, oss_file_name=None):
    """
    Load file from OSS to local directory.

    Args:
        oss_model_dir (str): the OSS model directory. It is in the format
            of oss://bucket/path/to/dir/.
        local_file_name (str): the local file path.
        oss_file_name (str): the OSS file path to load. If None,
            use local_file_name as oss_file_name.

    Returns:
        None.
    """
    if oss_file_name is None:
        oss_file_name = local_file_name

    oss_file_path = "/".join([oss_model_dir.rstrip("/"), oss_file_name])
    oss_file_path = remove_bucket_prefix(oss_file_path)
    bucket = get_models_bucket()
    bucket.get_object_to_file(oss_file_path, local_file_name)


def load_string(oss_file_path):
    data = load_bytes(oss_file_path)
    return data.decode("utf-8")


def load_bytes(oss_file_path):
    bucket = get_models_bucket()
    oss_file_path = remove_bucket_prefix(oss_file_path)
    return bucket.get_object(oss_file_path).read()


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

    # write a file "file_name_estimator" to store the estimator name,
    # so we can determine if the estimator is BoostedTrees* when
    # explaining the model.
    estimator_file_name = "_".join([file_name, "estimator"])
    oss_path = get_oss_path_from_uri(oss_model_dir, estimator_file_name)
    save_string(oss_path, meta[0])


def load_metas(oss_model_dir, file_name):
    '''Load model meta which are saved by save_metas from OSS

    Args:
        oss_model_dir: OSS URI that the model meta saved to.
        file_name: meta data file name

    Returns:
        A list contains the saved python objects
    '''
    oss_path = "/".join([oss_model_dir.rstrip("/"), file_name])
    serialized = load_bytes(oss_path)
    return pickle.loads(serialized)


def load_oss_model(oss_model_dir, estimator):
    is_estimator = is_tf_estimator(estimator)
    # Keras single node is using h5 format to save the model, no need to deal
    # with export model format. Keras distributed mode will use estimator, so
    # this is also needed.
    if is_estimator:
        load_file(oss_model_dir, "exported_path")

    # NOTE(typhoonzero): directory "model_save" is hardcoded in
    # codegen/tensorflow/codegen.go
    load_dir(os.path.join(oss_model_dir, "model_save"))


def save_oss_model(oss_model_dir, model_name, is_estimator,
                   feature_column_names, feature_column_names_map,
                   feature_metas, label_meta, model_params,
                   feature_columns_code, num_workers):
    # Keras single node is using h5 format to save the model, no need to deal
    # with export model format. Keras distributed mode will use estimator, so
    # this is also needed.
    if is_estimator:
        with open("exported_path", "rb") as fn:
            saved_model_path = fn.read()
        if isinstance(saved_model_path, bytes):
            saved_model_path = saved_model_path.decode("utf-8")
        save_dir(oss_model_dir, saved_model_path)
        save_file(oss_model_dir, "exported_path")
    else:
        if num_workers > 1:
            FLAGS = tf.app.flags.FLAGS
            if FLAGS.task_index == 0:
                save_file(oss_model_dir, "exported_path")
        else:
            save_dir(oss_model_dir, "model_save")

    save_metas(oss_model_dir, num_workers, "tensorflow_model_desc", model_name,
               feature_column_names, feature_column_names_map, feature_metas,
               label_meta, model_params, feature_columns_code)
