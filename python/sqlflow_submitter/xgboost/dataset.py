import sys
import os
import json
from sqlflow_submitter import db
from sqlflow_submitter.tensorflow.input_fn import pai_maxcompute_db_generator
import xgboost as xgb

SLICE_NUM=128

def xgb_dataset(datasource,
                fn,
                dataset_sql,
                feature_specs,
                feature_column_names,
                label_spec,
                is_pai=False,
                pai_table=""):

    if is_pai:
        pai_dataset(fn, feature_specs, feature_column_names, label_spec, "odps://{}/tables/{}".format(*pai_table.split(".")))
    else:
        conn = db.connect_with_data_source(datasource)
        gen = db.db_generator(conn.driver, conn, dataset_sql,
                              feature_column_names, label_spec, feature_specs)
        dump_dmatrix(fn, gen, label_spec)
    return xgb.DMatrix(fn)


def dump_dmatrix(filename, generator, has_label):
    # TODO(yancey1989): generate group and weight text file if necessary
    with open(filename, 'w') as f:
        for item in generator():
            if not has_label:
                row_data = ["%d:%f" % (i, v[0]) for i, v in enumerate(item[0])]
            else:
                features, label = item
                row_data = [str(label)] + [
                    "%d:%f" % (i, v[0]) for i, v in enumerate(features)
                ]
            f.write("\t".join(row_data) + "\n")


def pai_dataset(dir_name, feature_specs, feature_column_names, label_spec, pai_table):
    from subprocess import Popen, PIPE
    import threading
    threads = []
    os.mkdir(dir_name)

    def thread_worker(slice_id):
        p = Popen("{} -m {}".format(sys.executable, __name__),
                  shell=True, stdin=PIPE)
        p.communicate(
            json.dumps(
                [dir_name, feature_specs, feature_column_names, label_spec, pai_table, slice_id]))

    for i in range(SLICE_NUM):
        t = threading.Thread(target=thread_worker, args=(i,))
        t.start()
        threads.append(t)
    map(lambda t: t.join(), threads)


def pai_download_table_data_worker(dir_name, feature_specs, feature_column_names, label_spec, pai_table, slice_id):
    label_column_name = label_spec['feature_name'] if label_spec else None
    gen = pai_maxcompute_db_generator(
        pai_table, feature_column_names, label_column_name, feature_specs,
        slice_id=slice_id, slice_count=SLICE_NUM)
    with open("{}/{}".format(dir_name, slice_id), 'w') as f:
        for item in gen():
            row_data = ["%d:%f" % (i, v[0]) for i, v in enumerate(item[0])]
            if label_spec:
                row_data = [str(item[1])] + row_data
            f.write("\t".join(row_data) + "\n")


if __name__ == "__main__":
    pai_download_table_data_worker(*json.load(sys.stdin))
