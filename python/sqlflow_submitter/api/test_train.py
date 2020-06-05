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

from sqlflow_submitter.api import init
from sqlflow_submitter.api.train import train


def main():
    init(
        "mysql://root:root@tcp(sqlflow-mysql.default:3306)/?maxAllowedPacket=true"
    )
    train("SELECT * FROM iris.train",
          "DNNClassifier",
          "sqlflow_models.python_api_test_model",
          "class",
          attrs={
              "model.n_classes": "3",
              "train.batch_size": 8
          },
          validation_select="SELECT * FROM iris.test")


if __name__ == "__main__":
    main()
