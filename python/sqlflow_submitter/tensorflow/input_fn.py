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
from sqlflow_submitter.db import (connect_with_data_source, db_generator,
                                  parseMaxComputeDSN)


def parse_sparse_feature(features, label, feature_column_names, feature_metas):
    features_dict = dict()
    for idx, col in enumerate(features):
        name = feature_column_names[idx]
        if feature_metas[name]["is_sparse"]:
            i, v, s = col
            features_dict[name] = tf.SparseTensor(indices=i,
                                                  values=v,
                                                  dense_shape=s)
        else:
            features_dict[name] = col
    return features_dict, label


def get_dtype(type_str):
    if type_str == "float32":
        return tf.float32
    elif type_str == "int64":
        return tf.int64
    elif type_str == "string":
        return tf.string
    else:
        raise TypeError("not supported dtype: %s" % type_str)


def input_fn(select, conn, feature_column_names, feature_metas, label_meta):
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

    gen = db_generator(conn.driver, conn, select, feature_column_names,
                       label_meta["feature_name"], feature_metas)
    dataset = tf.data.Dataset.from_generator(
        gen, (tuple(feature_types), eval("tf.%s" % label_meta["dtype"])),
        (tuple(shapes), label_meta["shape"]))
    ds_mapper = functools.partial(parse_sparse_feature,
                                  feature_column_names=feature_column_names,
                                  feature_metas=feature_metas)
    return dataset.map(ds_mapper)


def pai_maxcompute_input_fn(pai_table,
                            datasource,
                            feature_column_names,
                            feature_metas,
                            label_meta,
                            num_workers=1,
                            worker_id=0,
                            map_to_dict=True):
    # NOTE(typhoonzero): datasource is only used to get current selected maxcompute project(database).
    table_parts = pai_table.split(".")
    if len(table_parts) == 2:
        database, table_name = table_parts
    elif len(table_parts) == 1:
        table_name = pai_table
        driver, dsn = datasource.split("://")
        database = parseMaxComputeDSN(dsn)[-1]
    else:
        raise ValueError("error database.table format: %s" % pai_table)

    tables = ["odps://%s/tables/%s" % (database, table_name)]
    record_defaults = []
    for name in feature_column_names:
        dtype = get_dtype(feature_metas[name]["dtype"])
        record_defaults.append(
            tf.constant(0, dtype=dtype, shape=feature_metas[name]["shape"]))
    record_defaults.append(
        tf.constant(0,
                    get_dtype(label_meta["dtype"]),
                    shape=label_meta["shape"]))

    selected_cols = copy.copy(feature_column_names)
    selected_cols.append(label_meta["feature_name"])
    if num_workers == 0:
        num_workers = 1
    dataset = tf.data.TableRecordDataset(tables,
                                         record_defaults=record_defaults,
                                         selected_cols=",".join(selected_cols),
                                         slice_id=worker_id,
                                         slice_count=num_workers)

    def tensor_to_dict(*args):
        num_features = len(feature_column_names)
        label = args[num_features]
        features_dict = dict()
        for idx in range(num_features):
            name = feature_column_names[idx]
            features_dict[name] = tf.reshape(args[idx], [-1])
        return features_dict, label

    def tensor_to_list(*args):
        num_features = len(feature_column_names)
        label = args[num_features]
        feature_list = []
        for f in args[:num_features]:
            feature_list.append(f.eval())
        return feature_list, label.eval()

    if map_to_dict:
        return dataset.map(tensor_to_dict)
    else:
        return dataset.as_numpy().map(tensor_to_list)


def pai_maxcompute_db_generator(table,
                                feature_column_names,
                                label_column_name,
                                feature_specs,
                                fetch_size=128):
    def read_feature(raw_val, feature_spec, feature_name):
        # FIXME(typhoonzero): Should use correct dtype here.
        if feature_spec["is_sparse"]:
            indices = np.fromstring(raw_val,
                                    dtype=int,
                                    sep=feature_spec["delimiter"])
            indices = indices.reshape(indices.size, 1)
            values = np.ones([indices.size], dtype=np.int32)
            dense_shape = np.array(feature_spec["shape"], dtype=np.int64)
            return (indices, values, dense_shape)
        else:
            # Dense string vector
            if feature_spec["delimiter"] != "":
                if feature_spec["dtype"] == "float32":
                    return np.fromstring(raw_val,
                                         dtype=float,
                                         sep=feature_spec["delimiter"])
                elif feature_spec["dtype"] == "int64":
                    return np.fromstring(raw_val,
                                         dtype=int,
                                         sep=feature_spec["delimiter"])
                else:
                    raise ValueError('unrecognize dtype {}'.format(
                        feature_spec[feature_name]["dtype"]))
            else:
                return (raw_val, )

    def reader():
        selected_cols = copy.copy(feature_column_names)
        if label_column_name:
            selected_cols.append(label_column_name)
            try:
                label_idx = selected_cols.index(label_column_name)
            except ValueError:
                # NOTE(typhoonzero): For clustering model, label_column_name may not in field_names when predicting.
                label_idx = None
        else:
            label_idx = None
        reader = tf.python_io.TableReader(
            table,
            selected_cols=",".join(selected_cols),
            slice_id=0,
            slice_count=1)
        while True:
            try:
                row = reader.read(num_records=1)[0]
                label = row[label_idx] if label_idx is not None else -1
                features = []
                for name in feature_column_names:
                    feature = read_feature(row[selected_cols.index(name)],
                                           feature_specs[name], name)
                    features.append(feature)
                yield tuple(features), label
            except Exception as e:
                reader.close()
                break

    return reader
