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
import json
import os

import runtime.temp_file as temp_file
from runtime.feature.column import (JSONDecoderWithFeatureColumn,
                                    JSONEncoderWithFeatureColumn)
from runtime.model import oss
from runtime.model.db import (read_with_generator_and_metadata,
                              write_with_generator_and_metadata)
from runtime.model.modelzoo import load_model_from_model_zoo
from runtime.model.tar import unzip_dir, zip_dir

# archive the current work director into a tarball
TARBALL_NAME = "model.tar.gz"

# serialize the Model object into file
MODEL_OBJ_FILE_NAME = "metadata.json"


class EstimatorType(object):
    """The enum type for various SQLFlow estimator.
    """
    # To stay compitable with old models, we start at 0
    TENSORFLOW = 0
    XGBOOST = 1
    # PAIML is the model type that trained by PAI machine learning algorithm
    # toolkit
    PAIML = 2


class Model(object):
    """Model module represents a SQLFlow trained model, which includes
    three parts:
    1. the estimator type indicates which SQLFlow estimator comes from.
    2. the model meta indicates the meta data of training .e.g attributions,
    feature column types.
    3. the model data indicated the trained model, which generated by the AI
    engine, .e.g TensorFlow, XGBoost.

    Usage:

        meta = runtime.model.collect_metadata(attributes={...}, ...)
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

    def get_type(self):
        """
        Returns the model type.
        """
        return self._typ

    def get_meta(self, name, default=None):
        """
        Get the metadata by name.

        Args:
            name (str): the metadata name.
            default: the default value if the name does not exist.

        Returns:
            Return the metadata with the given name if exists.
            Otherwise, return the default value.
        """
        return self._meta.get(name, default)

    def _to_dict(self):
        meta = dict(self._meta)
        meta["model_type"] = self._typ
        return meta

    @staticmethod
    def _from_dict(d):
        typ = d.pop("model_type")
        return Model(typ, d)

    def _zip(self, local_dir, tarball, save_to_db=False):
        """
        Zip the model information and all files in local_dir into a tarball.

        Args:
            local_dir (str): the local directory.
            tarball (str): the tarball path.

        Returns:
            None.
        """
        if not save_to_db:
            model_obj_file = os.path.join(local_dir, MODEL_OBJ_FILE_NAME)
            with open(model_obj_file, "w") as f:
                d = self._to_dict()
                f.write(json.dumps(d, cls=JSONEncoderWithFeatureColumn))
        else:
            model_obj_file = None

        zip_dir(local_dir, tarball, arcname="./")
        if model_obj_file:
            os.remove(model_obj_file)

    @staticmethod
    def _unzip(local_dir, tarball, load_from_db=False):
        """
        Unzip the tarball into local_dir and deserialize the model
        information.

        Args:
            local_dir (str): the local directory.
            tarball (str): the tarball path.

        Returns:
            Model: a Model object represent the model type and meta
            information.
        """
        unzip_dir(tarball, local_dir)
        if not load_from_db:
            model_obj_file = os.path.join(local_dir, MODEL_OBJ_FILE_NAME)
            with open(model_obj_file, "r") as f:
                d = json.loads(f.read(), cls=JSONDecoderWithFeatureColumn)
                model = Model._from_dict(d)
            os.remove(model_obj_file)
            return model

    def save_to_db(self, datasource, table, local_dir=None):
        """
        This save function would archive all the files on local_dir
        into a tarball, and save it into DBMS with the specified table
        name.

        Args:
            datasource (str): the connection string to DBMS.
            table (str): the saved table name.
            local_dir (str): the local directory to save.

        Returns:
            None.
        """
        if local_dir is None:
            local_dir = os.getcwd()

        with temp_file.TemporaryDirectory() as tmp_dir:
            tarball = os.path.join(tmp_dir, TARBALL_NAME)
            self._zip(local_dir, tarball, save_to_db=True)

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

            write_with_generator_and_metadata(datasource, table,
                                              _bytes_reader(tarball),
                                              self._to_dict())

    @staticmethod
    def load_from_db(datasource, table, local_dir=None):
        """
        Load the saved model from DBMS and unzip it on local_dir.

        Args:
            datasource (str): the connection string to DBMS
            table (str): the table name which saved in DBMS
            local_dir (str): the local directory to load.

        Returns:
            Model: a Model object represent the model type and meta
            information.
        """
        if local_dir is None:
            local_dir = os.getcwd()

        with temp_file.TemporaryDirectory() as tmp_dir:
            tarball = os.path.join(tmp_dir, TARBALL_NAME)
            idx = table.rfind('/')
            if idx >= 0:
                model_zoo_addr = table[0:idx]
                table = table[idx + 1:]
                idx = table.rfind(":")
                if idx >= 0:
                    table = table[0:idx]
                    tag = table[idx + 1:]
                else:
                    tag = ""

                metadata = load_model_from_model_zoo(model_zoo_addr, table,
                                                     tag, tarball)
            else:
                gen, metadata = read_with_generator_and_metadata(
                    datasource, table)
                with open(tarball, "wb") as f:
                    for data in gen():
                        f.write(bytes(data))

            Model._unzip(local_dir, tarball, load_from_db=True)

        return Model._from_dict(metadata)

    def save_to_oss(self, oss_model_dir, local_dir=None):
        """
        This save function would archive all the files on local_dir
        into a tarball, and save it into OSS model directory.

        Args:
            oss_model_dir (str): the OSS model directory to save.
                It is in the format of oss://bucket/path/to/dir/.
            local_dir (str): the local directory to save.

        Returns:
            None.
        """
        if local_dir is None:
            local_dir = os.getcwd()

        with temp_file.TemporaryDirectory() as tmp_dir:
            tarball = os.path.join(tmp_dir, TARBALL_NAME)
            self._zip(local_dir, tarball)
            oss.save_file(oss_model_dir, tarball, TARBALL_NAME)

    @staticmethod
    def load_from_oss(oss_model_dir, local_dir=None):
        """
        Load the saved model from OSS and unzip it on local_dir.

        Args:
            oss_model_dir (str): the OSS model directory to load.
                It is in the format of oss://bucket/path/to/dir/.
            local_dir (str): the local directory to load.

        Returns:
            Model: a Model object represent the model type and meta
            information.
        """
        if local_dir is None:
            local_dir = os.getcwd()

        with temp_file.TemporaryDirectory() as tmp_dir:
            tarball = os.path.join(tmp_dir, TARBALL_NAME)
            oss.load_file(oss_model_dir, tarball, TARBALL_NAME)
            return Model._unzip(local_dir, tarball)
