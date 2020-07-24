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
"""This module saves or loads the SQLFlow model.
"""
import os
from enum import Enum

from runtime.model.db import read_with_generator, write_with_generator
from runtime.model.tar import unzip_dir, zip_dir

try:
    import cPickle as pickle
except ModuleNotFoundError:
    import pickle

# archive the current work director into a tarball
tarball = "model.tar.gz"

# serialize the Model object into file
model_obj_file = "sqlflow_model.pkl"


class EstimatorType(Enum):
    """The enum type for various SQLFlow estimator.
    """
    # To stay compitable with old models, we start at 0
    TENSORFLOW = 0
    XGBOOST = 1
    # PAIML is the model type that trained by PAI machine learning algorithm toolkit
    PAIML = 2


class Model:
    """Model module represents a SQLFlow trained model, which includes
    three parts:
    1. the estimator type indicates which SQLFlow estimator comes from.
    2. the model meta indicates the meta data of training .e.g attributions,
    feature column types.
    3. the model data indicated the trained model, which generated by the AI
    engine, .e.g TensorFlow, XGBoost.

    Usage:

        meta = runtime.collect_model_metadata(train_params={...},
                                              model_params={...})
        m = runtime.model.Model(ModelType.XGBOOST, meta)
        m.save(datasource="mysql://", "sqlflow_models.my_model")

    """
    def __init__(self, typ, meta):
        """
        Args:
            typ: EstimatorType
                the enum value of EstimatorType.
            meta: JSON
                the training meta with JSON format.
        """
        self._typ = typ
        self._meta = meta
        self._dump_file = "sqlflow_model.pkl"

    def save(self, datasource, table, cwd="./"):
        """This save function would archive all the files on work director
        into a tarball, and saved it into DBMS with the specified table name.

        Args:
            datasource: string
                the connection string to DBMS.
            table: string
                the saved table name.
        """
        _dump_pkl(self, model_obj_file)
        zip_dir(cwd, tarball)

        def _bytes_reader(filename, buf_size=8 * 32):
            def _gen():
                with open(filename, "rb") as f:
                    while True:
                        data = f.read(buf_size)
                        if data:
                            yield data
                        else:
                            break

            return _gen

        write_with_generator(datasource, table, _bytes_reader(tarball))


def load(datasource, table, cwd="./"):
    """Load the saved model from DBMS and unzip it on the work director.

    Args:
        datasource: string
            The connection string to DBMS

        table: string
            The table name which saved in DBMS

    Returns:
        Model: a Model object represent the model type and meta information.
    """
    gen = read_with_generator(datasource, table)
    with open(tarball, "wb") as f:
        for data in gen():
            f.write(bytes(data))

    unzip_dir(tarball, cwd)
    return _load_pkl(os.path.join(cwd, model_obj_file))


def _dump_pkl(obj, to_file):
    """Dump the Python object to file with Pickle.
    """
    with open(to_file, "wb") as f:
        pickle.dump(obj, f, pickle.HIGHEST_PROTOCOL)


def _load_pkl(filename):
    """Load the Python object from a file with Pickle.
    """
    with open(filename, "rb") as f:
        return pickle.load(f)
