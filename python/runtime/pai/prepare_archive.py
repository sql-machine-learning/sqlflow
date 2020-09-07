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

import os
import pickle
import shutil
import subprocess
from os import path

from runtime.diagnostics import SQLFlowDiagnostic
from runtime.pai import pai_model
from runtime.pai.get_pai_tf_cmd import (ENTRY_FILE, JOB_ARCHIVE_FILE,
                                        PARAMS_FILE)

TRAIN_PARAMS_FILE = "train_params.pkl"

TF_REQUIREMENT = """
adanet==0.8.0
numpy==1.16.2
pandas==0.24.2
plotille==3.7
seaborn==0.9.0
shap==0.28.5
scikit-learn==0.20.4
tensorflow-datasets==3.0.0
"""

XGB_REQUIREMENT = TF_REQUIREMENT + """
xgboost==0.82
sklearn2pmml==0.56.0
sklearn_pandas==1.6.0
"""


def prepare_archive(cwd, estimator, model_save_path, train_params):
    """package needed resource into a tarball"""
    _create_pai_hyper_param_file(cwd, PARAMS_FILE, model_save_path)

    with open(path.join(cwd, TRAIN_PARAMS_FILE), "wb") as param_file:
        pickle.dump(train_params, param_file, protocol=2)

    with open(path.join(cwd, "requirements.txt"), "w") as require:
        require.write(_get_requirement(estimator))

    # copy entry.py to top level directory, so the package name `xgboost`
    # and `tensorflow` in runtime.pai will not conflict with the global ones
    shutil.copyfile(path.join(path.dirname(__file__), ENTRY_FILE),
                    path.join(cwd, ENTRY_FILE))
    _copy_python_package("runtime", cwd)
    _copy_python_package("sqlflow_models", cwd)
    _copy_custom_package(estimator, cwd)

    args = [
        "tar", "czf", JOB_ARCHIVE_FILE, ENTRY_FILE, "runtime",
        "sqlflow_models", "requirements.txt", TRAIN_PARAMS_FILE
    ]
    if subprocess.call(args, cwd=cwd) != 0:
        raise SQLFlowDiagnostic("Can't zip resource")


def _get_requirement(model_name):
    if model_name.lower().startswith("xgboost"):
        return XGB_REQUIREMENT
    else:
        return TF_REQUIREMENT


def _find_python_module_path(module):
    proc = os.popen("python -c \"import %s;print(%s.__path__[0])\"" %
                    (module, module))
    output = proc.readline()
    return output.strip()


def _copy_python_package(module, dest):
    module_path = _find_python_module_path(module)
    if not module_path:
        raise SQLFlowDiagnostic("Can't find module %s" % module)
    shutil.copytree(module_path, path.join(dest, path.basename(module_path)))


def _copy_custom_package(estimator, dst):
    model_name_parts = estimator.split(".")
    pkg_name = model_name_parts[0]
    if (len(model_name_parts) == 2 and pkg_name != "sqlflow_models"
            and pkg_name != "xgboost"):
        _copy_python_package(pkg_name, dst)


def _create_pai_hyper_param_file(cwd, filename, model_path):
    with open(path.join(cwd, filename), "w") as file:
        oss_ak = os.getenv("SQLFLOW_OSS_AK")
        oss_sk = os.getenv("SQLFLOW_OSS_SK")
        oss_ep = os.getenv("SQLFLOW_OSS_MODEL_ENDPOINT")
        if oss_ak == "" or oss_sk == "" or oss_ep == "":
            raise SQLFlowDiagnostic(
                "must define SQLFLOW_OSS_AK, SQLFLOW_OSS_SK, "
                "SQLFLOW_OSS_MODEL_ENDPOINT when submitting to PAI")
        file.write("sqlflow_oss_ak=\"%s\"\n" % oss_ak)
        file.write("sqlflow_oss_sk=\"%s\"\n" % oss_sk)
        file.write("sqlflow_oss_ep=\"%s\"\n" % oss_ep)
        oss_model_url = pai_model.get_oss_model_url(model_path)
        file.write("sqlflow_oss_modeldir=\"%s\"\n" % oss_model_url)
        file.flush()
