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


# NOTE(typhoonzero): FastPredict is used for predicting with Tensorflow Estimator,
# for more details, please checkout this blog: https://guillaumegenthial.github.io/serving-tensorflow-estimator.html
# Yet that implement may cause predict accuracy error, see: https://github.com/sql-machine-learning/sqlflow/issues/1397
# the fix is: https://github.com/sql-machine-learning/sqlflow/pull/1504.
class FastPredict:
    def __init__(self, estimator, input_fn):
        self.estimator = estimator
        self.input_fn = input_fn

    def predict(self, features):
        print("in predict:", features)

        def inner_func():
            print("inner_func yield:", features)
            yield features

        predictions = self.estimator.predict(
            input_fn=self.input_fn(inner_func))
        return [n for n in predictions]
