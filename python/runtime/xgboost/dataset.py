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
import os
import shutil
import sys
from pathlib import Path

import numpy as np
import runtime.feature.column as fc
import six
import xgboost as xgb
from runtime import db
from runtime.db import XGBOOST_NULL_MAGIC
from runtime.dbapi.paiio import PaiIOConnection
from runtime.feature.compile import compile_ir_feature_columns
from runtime.model import EstimatorType
from scipy.sparse import vstack
from sklearn.datasets import load_svmlight_file, load_svmlight_files

DMATRIX_FILE_SEP = "\t"


def xgb_dataset(datasource,
                fn,
                dataset_sql,
                feature_metas,
                feature_column_names,
                label_meta,
                is_pai=False,
                pai_table="",
                pai_single_file=False,
                cache=False,
                batch_size=None,
                epoch=1,
                rank=0,
                nworkers=1,
                transform_fn=None,
                feature_column_code="",
                raw_data_dir=None):
    if raw_data_dir:
        # raw_data_dir is needed when predicting. Because we
        # should write the raw data from the source db into
        # the dest db, instead of the transformed data after
        # `transform_fn(features)` . If raw_data_dir is not
        # None, the raw data from the source db would be written
        # into another file.
        if os.path.exists(raw_data_dir):
            shutil.rmtree(raw_data_dir, ignore_errors=True)

        os.mkdir(raw_data_dir)

    if is_pai:
        for dmatrix in pai_dataset(fn,
                                   feature_metas,
                                   feature_column_names,
                                   label_meta,
                                   pai_table,
                                   pai_single_file,
                                   cache,
                                   rank,
                                   nworkers,
                                   batch_size=batch_size,
                                   feature_column_code=feature_column_code,
                                   raw_data_dir=raw_data_dir):
            yield dmatrix
        return

    conn = db.connect_with_data_source(datasource)
    gen = db.db_generator(conn, dataset_sql, label_meta)()

    selected_cols = db.selected_cols(conn, dataset_sql)
    for _ in six.moves.range(epoch):
        step = 0
        # the filename per batch is [filename]_[step]
        step_file_name = "%s_%d" % (fn, step)
        written_rows = dump_dmatrix(step_file_name,
                                    gen,
                                    feature_column_names,
                                    feature_metas,
                                    label_meta,
                                    selected_cols,
                                    transform_fn=transform_fn,
                                    raw_data_dir=raw_data_dir)

        while written_rows > 0:
            yield load_dmatrix('{0}#{0}.cache'.format(step_file_name)
                               if cache else step_file_name)
            os.remove(step_file_name)

            step += 1
            step_file_name = "%s_%d" % (fn, step)
            written_rows = dump_dmatrix(step_file_name,
                                        gen,
                                        feature_column_names,
                                        feature_metas,
                                        label_meta,
                                        selected_cols,
                                        transform_fn=transform_fn,
                                        raw_data_dir=raw_data_dir)


def dump_dmatrix(filename,
                 generator,
                 feature_column_names,
                 feature_metas,
                 has_label,
                 selected_cols,
                 batch_size=None,
                 transform_fn=None,
                 raw_data_dir=None):
    # TODO(yancey1989): generate group and weight text file if necessary
    row_id = 0

    if raw_data_dir:
        index = filename.rindex('/') + 1 if '/' in filename else 0
        raw_data_fid = open(os.path.join(raw_data_dir, filename[index:]), 'a')
    else:
        raw_data_fid = None

    with open(filename, 'a') as f:
        for row, label in generator:
            features = db.read_features_from_row(row,
                                                 selected_cols,
                                                 feature_column_names,
                                                 feature_metas,
                                                 is_xgboost=True)

            if raw_data_fid is not None:
                raw_data_fid.write(
                    DMATRIX_FILE_SEP.join([str(r) for r in row]) + "\n")

            if transform_fn:
                features = transform_fn(features)

            row_data = []
            offset = 0
            for i, v in enumerate(features):
                if len(v) == 1:  # dense feature
                    value = v[0]
                    if isinstance(value, np.ndarray):
                        value = value.reshape((-1, ))
                        row_data.extend([
                            "{}:{}".format(i + offset, item)
                            for i, item in enumerate(value)
                        ])
                        offset += value.size
                    else:
                        row_data.append("{}:{}".format(offset, value))
                        offset += 1
                else:  # sparse feature
                    indices = v[0]
                    value = v[1].reshape((-1))
                    dense_size = np.prod(v[2])
                    row_data.extend([
                        "{}:{}".format(i + offset, item)
                        for i, item in six.moves.zip(indices, value)
                    ])
                    offset += dense_size

            if has_label:
                row_data = [str(label)] + row_data
            f.write(DMATRIX_FILE_SEP.join(row_data) + "\n")
            row_id += 1
            # batch_size == None means use all data in generator
            if batch_size is None:
                continue
            if row_id >= batch_size:
                break
    # return rows written
    if raw_data_fid is not None:
        raw_data_fid.close()

    return row_id


def load_dmatrix(filename):
    '''
    NOTE(sneaxiy): XGBoost distributed training using rabit would
    split CSV/LIBSVM file into N pieces automatically, where N is
    the worker number. However, in our implementation, we dump
    different data file into each worker, and each worker should
    not split the dumped file again when training. Otherwise,
    some data would be lost. To prevent the automatic data sharding
    by XGBoost itself, we load the LIBSVM file using
    'sklearn.datasets.load_svmlight_file' to be a CSR sparse matrix
    first, and then convert it to 'xgboost.DMatrix'.

    See https://github.com/sql-machine-learning/sqlflow/issues/2326
    in detailed.
    '''
    if xgb.rabit.get_world_size() > 1:
        # XGBoost DMatrix supports to load data from file path like
        # "train.txt#train.txt.cache". The actual data path is
        # "train.txt", while "train.txt.cache" is used as the
        # external memory cache. But "train.txt#train.txt.cache"
        # is not a valid file path, and it is not supported by
        # load_svmlight_file(s). So we remove the suffix "#..."
        # here before loading the data using load_svmlight_file(s).
        if '#' in filename:
            filename = filename[0:filename.index('#')]

        if os.path.isdir(filename):
            files = [os.path.join(filename, f) for f in os.listdir(filename)]
            assert len(files) > 0, "No data file found in {}".format(filename)

            ret = load_svmlight_files(files, zero_based=True)
            X = vstack(ret[0::2])
            y = np.concatenate(ret[1::2], axis=0)
            return xgb.DMatrix(X, y, missing=XGBOOST_NULL_MAGIC)
        else:
            ret = load_svmlight_file(filename, zero_based=True)
            return xgb.DMatrix(ret[0], ret[1], missing=XGBOOST_NULL_MAGIC)
    else:
        return xgb.DMatrix(filename, missing=XGBOOST_NULL_MAGIC)


def get_pai_table_slice_count(table, nworkers, batch_size):
    if batch_size is None or batch_size <= 0:
        batch_size = 4096  # default batch_size

    row_cnt = PaiIOConnection.from_table(table).get_table_row_num()

    assert row_cnt >= nworkers, "Data number {} should not " \
                                "less than worker number {}"\
        .format(row_cnt, nworkers)

    slice_num_per_worker = max(int(row_cnt / (nworkers * batch_size)), 1)
    slice_count = slice_num_per_worker * nworkers

    print('row_cnt = {}, slice_count = {}, nworkers = {}'.format(
        row_cnt, slice_count, nworkers))

    return slice_count


def pai_dataset(filename,
                feature_metas,
                feature_column_names,
                label_meta,
                pai_table,
                single_file,
                cache,
                rank=0,
                nworkers=1,
                batch_size=None,
                feature_column_code="",
                raw_data_dir=None):

    from subprocess import Popen, PIPE
    from multiprocessing.dummy import Pool  # ThreadPool
    import queue
    dname = filename
    if single_file:
        dname = filename + '.dir'
    if os.path.exists(dname):
        shutil.rmtree(dname, ignore_errors=True)

    os.mkdir(dname)
    slice_count = get_pai_table_slice_count(pai_table, nworkers, batch_size)
    thread_num = min(int(slice_count / nworkers), 128)
    pool = Pool(thread_num)
    complete_queue = queue.Queue()

    def thread_worker(slice_id):
        # add universal_newlines=True to be compatible with Python3.
        p = Popen("{} -m {}".format(sys.executable, __name__),
                  shell=True,
                  stdin=PIPE,
                  universal_newlines=True)
        p.communicate(
            json.dumps([
                dname, feature_metas, feature_column_names, label_meta,
                pai_table, slice_id, slice_count, feature_column_code,
                raw_data_dir
            ],
                       cls=fc.JSONEncoderWithFeatureColumn))
        assert p.returncode == 0, \
            "The subprocess raises error when reading data"
        complete_queue.put(slice_id)

    slice_id = rank
    slice_total = 0
    while slice_id < slice_count:
        pool.apply_async(thread_worker, (slice_id, ))
        slice_id += nworkers
        slice_total += 1

    if batch_size is None:
        pool.close()
        pool.join()
        yield load_dmatrix('{0}#{0}.cache'.format(dname) if cache else dname)
        return

    for _ in six.moves.range(slice_total):
        slice_id = complete_queue.get(block=True)
        if not single_file:
            downloaded_file = "./{}/{}.txt".format(dname, slice_id)
            # ignore empty files or the xgb.DMatrix will throw error.
            if Path(downloaded_file).stat().st_size > 0:
                yield load_dmatrix('{0}#{0}.cache'.format(downloaded_file)
                                   if cache else downloaded_file)
                os.unlink(downloaded_file)

    if single_file:

        def merge_files(dir_name, file_name):
            cmd = "cat %s/*.txt > %s" % (dir_name, file_name)
            p = Popen(cmd, shell=True, stdin=PIPE, stderr=PIPE)
            out, err = p.communicate()
            if err:
                raise Exception("merge data files failed: %s" % err)

        merge_files(dname, filename)
        if raw_data_dir:
            merge_files(raw_data_dir, '{}.raw'.format(filename))

        yield load_dmatrix(
            '{0}#{0}.cache'.format(filename) if cache else filename)

    pool.close()
    pool.join()


def pai_download_table_data_worker(dname, feature_metas, feature_column_names,
                                   label_meta, pai_table, slice_id,
                                   slice_count, feature_column_code,
                                   raw_data_dir):
    import runtime.xgboost as xgboost_extended
    if isinstance(feature_column_code, dict):
        # NOTE(typhoonzero): feature_column_code is a dict of
        # runtime.feature.column in refactored step code.
        feature_column_transformers = compile_ir_feature_columns(
            feature_column_code, EstimatorType.XGBOOST)
        transform_fn = \
            xgboost_extended.feature_column.ComposedColumnTransformer(
                feature_column_names,
                *feature_column_transformers["feature_columns"])
    else:
        feature_column_transformers = eval('[{}]'.format(feature_column_code))
        transform_fn = \
            xgboost_extended.feature_column.ComposedColumnTransformer(
                feature_column_names, *feature_column_transformers)

    conn = PaiIOConnection.from_table(pai_table, slice_id, slice_count)
    gen = db.db_generator(conn, None, label_meta=label_meta)()
    selected_cols = db.selected_cols(conn, None)
    filename = "{}/{}.txt".format(dname, slice_id)
    dump_dmatrix(filename,
                 gen,
                 feature_column_names,
                 feature_metas,
                 label_meta,
                 selected_cols,
                 transform_fn=transform_fn,
                 raw_data_dir=raw_data_dir)


if __name__ == "__main__":
    pai_download_table_data_worker(
        *json.load(sys.stdin, cls=fc.JSONDecoderWithFeatureColumn))
