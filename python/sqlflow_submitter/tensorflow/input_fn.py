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

import copy
import functools

import numpy as np
import tensorflow as tf
from sqlflow_submitter import db


def parse_sparse_feature(features, label, feature_column_names, feature_metas):
    features_dict = dict()
    for idx, col in enumerate(features):
        name = feature_column_names[idx]
        if feature_metas[name]["is_sparse"]:
            features_dict[name] = tf.SparseTensor(*col)
        else:
            features_dict[name] = col
    return features_dict, label


def parse_sparse_feature_predict(features, feature_column_names,
                                 feature_metas):
    features_dict = dict()
    for idx, col in enumerate(features):
        name = feature_column_names[idx]
        if feature_metas[name]["is_sparse"]:
            features_dict[name] = tf.SparseTensor(*col)
        else:
            features_dict[name] = col
    return features_dict


def get_dtype(type_str):
    if type_str == "float32":
        return tf.float32
    elif type_str == "int64":
        return tf.int64
    elif type_str == "string":
        return tf.string
    else:
        raise TypeError("not supported dtype: %s" % type_str)


def input_fn(select,
             datasource,
             feature_column_names,
             feature_metas,
             label_meta,
             is_pai=False,
             pai_table="",
             num_workers=1,
             worker_id=0):
    feature_types = []
    shapes = []
    for name in feature_column_names:
        # NOTE: vector columns like 23,21,3,2,0,0 should use shape None
        if feature_metas[name]["is_sparse"]:
            feature_types.append((tf.int64, tf.int32, tf.int64))
            shapes.append((None, None, None))
        else:
            feature_types.append(get_dtype(feature_metas[name]["dtype"]))
            shapes.append(feature_metas[name]["shape"])
    if is_pai:
        return pai_dataset("odps://{}/tables/{}".format(*pai_table.split(".")),
                           feature_column_names,
                           label_meta,
                           feature_metas,
                           slice_id=worker_id,
                           slice_count=num_workers)
    else:
        conn = db.connect_with_data_source(datasource)
        gen = db.db_generator(conn.driver, conn, select, feature_column_names,
                              label_meta, feature_metas)
    # Clustering model do not have label
    if not label_meta or label_meta["feature_name"] == "":
        dataset = tf.data.Dataset.from_generator(gen, (tuple(feature_types), ),
                                                 (tuple(shapes), ))
        ds_mapper = functools.partial(
            parse_sparse_feature_predict,
            feature_column_names=feature_column_names,
            feature_metas=feature_metas)
    else:
        dataset = tf.data.Dataset.from_generator(
            gen, (tuple(feature_types), eval("tf.%s" % label_meta["dtype"])),
            (tuple(shapes), label_meta["shape"]))
        ds_mapper = functools.partial(
            parse_sparse_feature,
            feature_column_names=feature_column_names,
            feature_metas=feature_metas)
    return dataset.map(ds_mapper)


def parse_pai_dataset(feature_column_names, has_label, feature_specs, *row):
    features = {}
    for i, name in enumerate(feature_column_names):
        spec = feature_specs[name]
        f = db.read_feature(row[i], spec, name)
        features[name] = tf.SparseTensor(*f) if spec["is_sparse"] else list(f)
    return features, row[-1] if has_label else features


def pai_dataset(table,
                feature_column_names,
                label_spec,
                feature_specs,
                slice_id=0,
                slice_count=1):
    record_defaults = []
    selected_cols = copy.copy(feature_column_names)
    dtypes = [feature_specs[n]["dtype"] for n in feature_column_names]
    if label_spec and label_spec["feature_name"]:
        selected_cols.append(label_spec["feature_name"])
        dtypes.append(label_spec["dtype"])

    import paiio
    return paiio.TableRecordDataset(
        table, ["" if t == "string" else eval("np.%s()" % t) for t in dtypes],
        selected_cols=",".join(selected_cols),
        slice_id=slice_id,
        slice_count=slice_count,
        capacity=2**25,
        num_threads=64).map(
            functools.partial(parse_pai_dataset, feature_column_names,
                              label_spec["feature_name"], feature_specs))
