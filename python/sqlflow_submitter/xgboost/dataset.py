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
import sys
from pathlib import Path

import numpy as np
import six
import xgboost as xgb
from scipy.sparse import vstack
from sklearn.datasets import load_svmlight_file, load_svmlight_files
from sqlflow_submitter import db


def xgb_dataset(datasource,
                fn,
                dataset_sql,
                feature_specs,
                feature_column_names,
                label_spec,
                is_pai=False,
                pai_table="",
                pai_single_file=False,
                cache=False,
                batch_size=None,
                epoch=1,
                rank=0,
                nworkers=1,
                transform_fn=None,
                feature_column_code=""):
    if is_pai:
        for dmatrix in pai_dataset(
                fn,
                feature_specs,
                feature_column_names,
                label_spec,
                "odps://{}/tables/{}".format(*pai_table.split(".")),
                pai_single_file,
                cache,
                rank,
                nworkers,
                batch_size=batch_size,
                feature_column_code=feature_column_code):
            yield dmatrix
        return

    conn = db.connect_with_data_source(datasource)
    gen = db.db_generator(conn.driver, conn, dataset_sql, feature_column_names,
                          label_spec, feature_specs)()

    selected_cols = db.selected_cols(conn.driver, conn, dataset_sql)
    for _ in six.moves.range(epoch):
        step = 0
        # the filename per batch is [filename]_[step]
        step_file_name = "%s_%d" % (fn, step)
        written_rows = dump_dmatrix(step_file_name,
                                    gen,
                                    feature_column_names,
                                    feature_specs,
                                    label_spec,
                                    selected_cols,
                                    transform_fn=transform_fn)

        while written_rows > 0:
            yield load_dmatrix('{0}#{0}.cache'.format(step_file_name)
                               if cache else step_file_name)
            os.remove(step_file_name)

            step += 1
            step_file_name = "%s_%d" % (fn, step)
            written_rows = dump_dmatrix(step_file_name,
                                        gen,
                                        feature_column_names,
                                        feature_specs,
                                        label_spec,
                                        selected_cols,
                                        transform_fn=transform_fn)


def dump_dmatrix(filename,
                 generator,
                 feature_column_names,
                 feature_specs,
                 has_label,
                 selected_cols,
                 batch_size=None,
                 transform_fn=None):
    # TODO(yancey1989): generate group and weight text file if necessary
    row_id = 0
    with open(filename, 'a') as f:
        for row, label in generator:
            features = db.read_features_from_row(row, selected_cols,
                                                 feature_column_names,
                                                 feature_specs)

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
                    row_data.extend(
                        "{}:{}".format(i + offset, item)
                        for i, item in six.moves.zip(indices, value))
                    offset += dense_size

            if has_label:
                row_data = [str(label)] + row_data

            f.write("\t".join(row_data) + "\n")
            row_id += 1
            # batch_size == None meas use all data in generator
            if batch_size == None:
                continue
            if row_id >= batch_size:
                break
    # return rows written
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
        if os.path.isdir(filename):
            files = [os.path.join(filename, f) for f in os.listdir(filename)]
            ret = load_svmlight_files(files, zero_based=True)
            X = vstack(ret[0::2])
            y = np.concatenate(ret[1::2], axis=0)
            return xgb.DMatrix(X, y)
        else:
            ret = load_svmlight_file(filename, zero_based=True)
            return xgb.DMatrix(ret[0], ret[1])
    else:
        return xgb.DMatrix(filename)


def get_pai_table_slice_count(table, nworkers, batch_size):
    if batch_size is None or batch_size <= 0:
        batch_size = 4096  # default batch_size

    row_cnt = db.get_pai_table_row_num(table)

    assert row_cnt >= nworkers, "Data number {} should not less than worker number {}".format(
        row_cnt, nworkers)

    slice_num_per_worker = max(int(row_cnt / (nworkers * batch_size)), 1)
    slice_count = slice_num_per_worker * nworkers

    print('row_cnt = {}, slice_count = {}, nworkers = {}'.format(
        row_cnt, slice_count, nworkers))

    return slice_count


def pai_dataset(filename,
                feature_specs,
                feature_column_names,
                label_spec,
                pai_table,
                single_file,
                cache,
                rank=0,
                nworkers=1,
                batch_size=None,
                feature_column_code=""):
    from subprocess import Popen, PIPE
    from multiprocessing.dummy import Pool  # ThreadPool
    import queue

    dname = filename
    if single_file:
        dname = filename + '.dir'
    os.mkdir(dname)

    slice_count = get_pai_table_slice_count(pai_table, nworkers, batch_size)

    thread_num = min(int(slice_count / nworkers), 128)

    pool = Pool(thread_num)
    complete_queue = queue.Queue()

    def thread_worker(slice_id):
        p = Popen("{} -m {}".format(sys.executable, __name__),
                  shell=True,
                  stdin=PIPE)
        p.communicate(
            json.dumps([
                dname, feature_specs, feature_column_names, label_spec,
                pai_table, slice_id, slice_count, feature_column_code
            ]))
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
        cmd = "cat %s/*.txt > %s" % (dname, filename)
        p = Popen(cmd, shell=True, stdin=PIPE, stderr=PIPE)
        out, err = p.communicate()
        if err:
            raise Exception("merge data files failed: %s" % err)
        yield load_dmatrix(
            '{0}#{0}.cache'.format(filename) if cache else filename)

    pool.close()
    pool.join()


def pai_download_table_data_worker(dname, feature_specs, feature_column_names,
                                   label_spec, pai_table, slice_id,
                                   slice_count, feature_column_code):
    import sqlflow_submitter.xgboost as xgboost_extended
    feature_column_transformers = eval('list({})'.format(feature_column_code))
    transform_fn = xgboost_extended.feature_column.ComposedColumnTransformer(
        feature_column_names, *feature_column_transformers)

    label_column_name = label_spec['feature_name'] if label_spec else None
    gen = db.pai_maxcompute_db_generator(pai_table,
                                         feature_column_names,
                                         label_column_name,
                                         feature_specs,
                                         slice_id=slice_id,
                                         slice_count=slice_count)()
    selected_cols = db.pai_selected_cols(pai_table)
    filename = "{}/{}.txt".format(dname, slice_id)
    dump_dmatrix(filename,
                 gen,
                 feature_column_names,
                 feature_specs,
                 label_spec,
                 selected_cols,
                 transform_fn=transform_fn)


if __name__ == "__main__":
    pai_download_table_data_worker(*json.load(sys.stdin))
