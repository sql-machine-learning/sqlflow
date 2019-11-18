# Copyright 2019 The SQLFlow Authors. All rights reserved.
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

import io
import pickle
import tarfile
import odps
import tensorflow as tf
from sqlflow_submitter import db

def save(datasource, name, model_dir, *meta):
    o = db.connect_with_data_source(datasource)
    o.delete_table(name, if_exists=True)
    t = o.create_table(name, 'piece binary')
    f = io.BytesIO()
    archive = tarfile.open(None, "w|gz", f)
    archive.add(model_dir)
    archive.close()
    f.seek(0)

    with t.open_writer() as w:
        w.write([pickle.dumps([model_dir] + list(meta))])
        w.write(list(iter(lambda:[f.read(8000000)], [b''])))

def load(datasource, name):
    o = db.connect_with_data_source(datasource)
    t = o.get_table(name)
    f = io.BytesIO()
    with t.open_reader() as r:
        meta = pickle.loads(r[0]['piece'])
        for record in r[1:]:
            f.write(record['piece'])
    f.seek(0)
    archive = tarfile.open(None, "r|gz", f)
    archive.extractall()
    archive.close()
    return meta
