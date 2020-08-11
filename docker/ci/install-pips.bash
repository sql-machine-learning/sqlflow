#!/bin/bash

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

set -e

# keras.datasets.imdb only works with numpy==1.16.1
# NOTE: shap == 0.30.1 depends on dill but not include dill as it's dependency,
#       need to install manually.
# NOTE: mysqlclient depends on apt package mysqlclient from install-mysql.bash.
# NOTE: Currently, jpmml-evaluator is only needed in unittest to verify whether
#       the saved PMML file is right.
pip install --quiet \
    numpy==1.16.2 \
    tensorflow-metadata==0.22.2 \
    tensorflow==2.0.1 \
    impyla==0.16.0 \
    pyodps==0.8.3 \
    dill==0.3.0 \
    shap==0.30.1 \
    xgboost==0.90 \
    oss2==2.9.0 \
    plotille==3.7 \
    seaborn==0.9.0 \
    scikit-learn==0.21.0 \
    sklearn2pmml==0.56.0 \
    jpmml-evaluator==0.3.1 \
    PyUtilib==5.8.0 \
    pyomo==5.6.9 \
    pyodps==0.8.3 \
    requests==2.23.0

