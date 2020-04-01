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
                pai_single_file=False,
                cache=False,
                batch_size=None,
                epoch=1):

    if is_pai:
        for dmatrix in pai_dataset(
                fn,
                feature_specs,
                feature_column_names,
                label_spec,
                "odps://{}/tables/{}".format(*pai_table.split(".")),
                pai_single_file,
                cache,
                batch_size=batch_size):
            yield dmatrix
        return

    conn = db.connect_with_data_source(datasource)
    gen = db.db_generator(conn.driver, conn, dataset_sql, feature_column_names,
                          label_spec, feature_specs)()

    for i in range(epoch):
        step = 0
        # the filename per batch is [filename]_[step]
        step_file_name = "%s_%d" % (fn, step)
        writed_rows = dump_dmatrix(step_file_name, gen, label_spec)

        while writed_rows > 0:
            yield xgb.DMatrix('{0}#{0}.cache'.format(step_file_name)
                              if cache else step_file_name)
            os.remove(step_file_name)

            step += 1
            step_file_name = "%s_%d" % (fn, step)
            writed_rows = dump_dmatrix(step_file_name, gen, label_spec)


def dump_dmatrix(filename, generator, has_label, batch_size=None):
    # TODO(yancey1989): generate group and weight text file if necessary
    row_id = 0
    with open(filename, 'a') as f:
        for item in generator:
            row_data = [
                "%d:%f" % (i, v[0] or 0) for i, v in enumerate(item[0])
            ]
            if has_label:
                row_data = [str(item[1])] + row_data
            f.write("\t".join(row_data) + "\n")
            row_id += 1
            # batch_size == None meas use all data in generator
            if batch_size == None:
                continue
            if row_id >= batch_size:
                break
    # return rows writed
    return row_id


def pai_dataset(filename,
                feature_specs,
                feature_column_names,
                label_spec,
                pai_table,
                single_file,
                cache,
                batch_size=None):
    from subprocess import Popen, PIPE
    import threading
    import queue
    threads = []
    complete_queue = queue.Queue()

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
        complete_queue.put(slice_id)

    for i in range(SLICE_NUM):
        t = threading.Thread(target=thread_worker, args=(i, ))
        t.start()
        threads.append(t)

    # map(lambda t: t.join(), threads)

    # Use all data at once if batch size == None, else use a static SLICE_NUM
    # FIXME(typhoonzero): pai xgboost only support fixed SLICE_NUM now.
    if batch_size == None:
        map(lambda t: t.join(), threads)
        yield xgb.DMatrix('{0}#{0}.cache'.format(dname) if cache else dname)
        return

    downloaded_slice_count = 0
    while True:
        slice_id = complete_queue.get(block=True)
        downloaded_slice_count += 1
        if downloaded_slice_count == SLICE_NUM:
            break
        if not single_file:
            downloaded_file = "./{}/{}.txt".format(dname, slice_id)
            # ignore empty files or the xgb.DMatrix will throw error.
            if Path(downloaded_file).stat().st_size == 0:
                continue
            yield xgb.DMatrix('{0}#{0}.cache'.format(downloaded_file)
                              if cache else downloaded_file)
            os.unlink(downloaded_file)

    if single_file:
        cmd = "cat %s/*.txt > %s" % (dname, filename)
        p = Popen(cmd, shell=True, stdin=PIPE, stderr=PIPE)
        out, err = p.communicate()
        if err:
            raise Exception("merge data files failed: %s" % err)
        yield xgb.DMatrix(
            '{0}#{0}.cache'.format(filename) if cache else filename)


def pai_download_table_data_worker(dname, feature_specs, feature_column_names,
                                   label_spec, pai_table, slice_id):
    label_column_name = label_spec['feature_name'] if label_spec else None
    gen = db.pai_maxcompute_db_generator(pai_table,
                                         feature_column_names,
                                         label_column_name,
                                         feature_specs,
                                         slice_id=slice_id,
                                         slice_count=SLICE_NUM)()
    filename = "{}/{}.txt".format(dname, slice_id)
    dump_dmatrix(filename, gen, label_spec)


if __name__ == "__main__":
    pai_download_table_data_worker(*json.load(sys.stdin))
