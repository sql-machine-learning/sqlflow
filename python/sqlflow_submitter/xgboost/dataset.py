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

import xgboost as xgb
from sqlflow_submitter import db

SLICE_NUM = 128


def xgb_dataset(datasource,
                fn,
                dataset_sql,
                feature_specs,
                feature_column_names,
                label_spec,
                is_pai=False,
                pai_table="",
                pai_single_file=False):

    if is_pai:
        return pai_dataset(fn, feature_specs, feature_column_names, label_spec,
                           "odps://{}/tables/{}".format(*pai_table.split(".")),
                           pai_single_file)
    conn = db.connect_with_data_source(datasource)
    gen = db.db_generator(conn.driver, conn, dataset_sql, feature_column_names,
                          label_spec, feature_specs)
    dump_dmatrix(fn, gen, label_spec)
    return xgb.DMatrix(fn)


def dump_dmatrix(filename, generator, has_label):
    # TODO(yancey1989): generate group and weight text file if necessary
    with open(filename, 'a') as f:
        for item in generator():
            row_data = [
                "%d:%f" % (i, v[0] or 0) for i, v in enumerate(item[0])
            ]
            if has_label:
                row_data = [str(item[1])] + row_data
            f.write("\t".join(row_data) + "\n")


def pai_dataset(filename, feature_specs, feature_column_names, label_spec,
                pai_table, single_file):
    from subprocess import Popen, PIPE
    import threading
    threads = []

    dname = filename
    if single_file:
        dname = filename + '.dir'
    os.mkdir(dname)

    def thread_worker(slice_id):
        p = Popen("{} -m {}".format(sys.executable, __name__),
                  shell=True,
                  stdin=PIPE)
        p.communicate(
            json.dumps([
                dname, feature_specs, feature_column_names, label_spec,
                pai_table, slice_id
            ]))

    for i in range(SLICE_NUM):
        t = threading.Thread(target=thread_worker, args=(i, ))
        t.start()
        threads.append(t)
    map(lambda t: t.join(), threads)

    if single_file:
        import glob
        filenames = glob.glob('%s/*.txt' % dname)
        with open(filename, 'w') as fw:
            for fname in filenames:
                with open(fname) as fr:
                    for line in fr:
                        fw.write(line)
        return xgb.DMatrix(filename)
    return xgb.DMatrix(dname)


def pai_download_table_data_worker(dname, feature_specs, feature_column_names,
                                   label_spec, pai_table, slice_id):
    label_column_name = label_spec['feature_name'] if label_spec else None
    gen = db.pai_maxcompute_db_generator(pai_table,
                                         feature_column_names,
                                         label_column_name,
                                         feature_specs,
                                         slice_id=slice_id,
                                         slice_count=SLICE_NUM)
    filename = "{}/{}.txt".format(dname, slice_id)
    dump_dmatrix(filename, gen, label_spec)


if __name__ == "__main__":
    pai_download_table_data_worker(*json.load(sys.stdin))
