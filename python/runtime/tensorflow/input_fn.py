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
from runtime import db


def parse_sparse_feature(features, label, feature_column_names, feature_metas):
    features_dict = dict()
    for idx, col in enumerate(features):
        name = feature_column_names[idx]
        if feature_metas[name]["is_sparse"]:
            # NOTE(sneaxiy): be careful that not all feature column APIs accept
            # SparseTensor.
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


def tf_generator(gen, selected_cols, feature_column_names, feature_metas):
    def reader():
        for row, label in gen():
            features = db.read_features_from_row(row, selected_cols,
                                                 feature_column_names,
                                                 feature_metas)
            features = list(features)
            for i, f in enumerate(features):
                if len(f) == 1 and isinstance(f[0], np.ndarray):
                    features[i] = f[0]
            features = tuple(features)

            if label is None:
                yield (features, )
            else:
                yield (features, label)

    return reader


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
        pai_table = "odps://{}/tables/{}".format(*pai_table.split("."))
        return pai_dataset(pai_table,
                           feature_column_names,
                           label_meta,
                           feature_metas,
                           slice_id=worker_id,
                           slice_count=num_workers)
    else:
        conn = db.connect_with_data_source(datasource)
        gen = db.db_generator(conn, select, label_meta)
        selected_cols = db.selected_cols(conn, select)

    gen = tf_generator(gen, selected_cols, feature_column_names, feature_metas)

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


def read_feature_as_tensor(raw_val, feature_spec, feature_name):
    # FIXME(typhoonzero): Should use correct dtype here.
    if feature_spec["delimiter"] == "":
        return [raw_val]
    if feature_spec["is_sparse"]:
        indices = tf.strings.to_number(
            tf.strings.split(raw_val,
                             feature_spec["delimiter"],
                             result_type='RaggedTensor'), tf.int64)
        values = tf.fill(tf.shape(indices), 1)
        indices = tf.expand_dims(indices, 1)
        dense_shape = np.array(feature_spec["shape"], dtype=np.int64)
        return (indices, values, dense_shape)
    else:  # Dense string vector
        return tf.strings.to_number(
            tf.strings.split(raw_val,
                             feature_spec["delimiter"],
                             result_type='RaggedTensor'),
            feature_spec["dtype"])


def parse_pai_dataset(feature_column_names, label_meta, feature_metas, *row):
    features = {}
    for i, name in enumerate(feature_column_names):
        spec = feature_metas[name]
        f = read_feature_as_tensor(row[i], spec, name)
        features[name] = tf.SparseTensor(*f) if spec["is_sparse"] else f
    label = row[-1] if label_meta["feature_name"] else -1
    if label_meta and label_meta["delimiter"] != "":
        # FIXME(typhoonzero): the label in the yielded row may not be the last
        # item, should get label index.
        tmp = tf.strings.split(label,
                               sep=label_meta["delimiter"],
                               result_type='RaggedTensor')
        if label_meta["dtype"] == "float32":
            label = tf.strings.to_number(tmp, out_type=tf.dtypes.float32)
        elif label_meta["dtype"] == "int64":
            label = tf.strings.to_number(tmp, out_type=tf.dtypes.int64)

    return features, label


def pai_dataset(table,
                feature_column_names,
                label_meta,
                feature_metas,
                slice_id=0,
                slice_count=1):
    selected_cols = copy.copy(feature_column_names)
    dtypes = [
        "string"
        if feature_metas[n]["delimiter"] else feature_metas[n]["dtype"]
        for n in feature_column_names
    ]
    if label_meta and label_meta["feature_name"]:
        selected_cols.append(label_meta["feature_name"])
        if label_meta["delimiter"] != "":
            dtypes.append("string")
        else:
            dtypes.append(label_meta["dtype"])

    import paiio
    return paiio.TableRecordDataset(
        table, ["" if t == "string" else eval("np.%s()" % t) for t in dtypes],
        selected_cols=",".join(selected_cols),
        slice_id=slice_id,
        slice_count=slice_count,
        capacity=2**25,
        num_threads=64).map(
            functools.partial(parse_pai_dataset, feature_column_names,
                              label_meta, feature_metas))


def get_dataset_fn(select,
                   datasource,
                   feature_column_names,
                   feature_metas,
                   label_meta,
                   is_pai,
                   pai_table,
                   batch_size,
                   epochs=1,
                   shuffle_size=None,
                   num_workers=1,
                   worker_id=0):
    def dataset_input_fn():
        dataset = input_fn(select,
                           datasource,
                           feature_column_names,
                           feature_metas,
                           label_meta,
                           is_pai=is_pai,
                           pai_table=pai_table,
                           num_workers=num_workers,
                           worker_id=worker_id)
        # NOTE(typhoonzero): on PAI some times cache to a file may cause
        # "lockfile already exists" error.
        dataset = dataset.cache()
        if shuffle_size is not None:
            dataset = dataset.shuffle(shuffle_size)
        dataset = dataset.batch(batch_size)
        if epochs > 1:
            dataset = dataset.repeat(epochs)
        return dataset

    return dataset_input_fn
