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
                                  parseMaxComputeDSN, read_feature)

try:
    import paiio
except:
    pass


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


def parse_sparse_feature_predict(features, feature_column_names,
                                 feature_metas):
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
        pai_table_parts = pai_table.split(".")
        formated_pai_table = "odps://%s/tables/%s" % (pai_table_parts[0],
                                                      pai_table_parts[1])
        gen = pai_maxcompute_db_generator(formated_pai_table,
                                          feature_column_names,
                                          label_meta["feature_name"],
                                          feature_metas,
                                          slice_id=worker_id,
                                          slice_count=num_workers)
    else:
        conn = connect_with_data_source(datasource)
        gen = db_generator(conn.driver, conn, select, feature_column_names,
                           label_meta, feature_metas)
    # Clustering model do not have label
    if label_meta["feature_name"] == "":
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


def pai_maxcompute_db_generator(table,
                                feature_column_names,
                                label_column_name,
                                feature_specs,
                                fetch_size=128,
                                slice_id=0,
                                slice_count=1):
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
        reader = paiio.TableReader(table,
                                   selected_cols=",".join(selected_cols),
                                   slice_id=slice_id,
                                   slice_count=slice_count)
        while True:
            try:
                row = reader.read(num_records=1)[0]
                label = row[label_idx] if label_idx is not None else -1
                features = []
                for name in feature_column_names:
                    feature = read_feature(row[selected_cols.index(name)],
                                           feature_specs[name], name)
                    features.append(feature)
                if label_column_name:
                    yield tuple(features), label
                else:
                    yield (tuple(features), )
            except Exception as e:
                reader.close()
                break

    return reader
