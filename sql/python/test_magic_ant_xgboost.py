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

import sys
from io import StringIO
from IPython import get_ipython
import unittest


ipython = get_ipython()


class TestSQLFlowMagic(unittest.TestCase):
    train_statement = """
SELECT *
FROM iris.train
TRAIN antxgboost.Estimator
WITH
	train.objective = "multi:softprob",
	train.num_class = 3,
	train.max_depth = 5,
	train.eta = 0.3,
	train.tree_method = "approx",
	train.num_round = 30
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class INTO sqlflow_models.my_xgboost_model;
"""
    pred_statement = """
SELECT *
FROM iris.test
PREDICT iris.predict.result
WITH
	pred.append_columns = [sepal_length, sepal_width, petal_length, petal_width],
	pred.prob_column = prob,
	pred.detail_column = detail
USING sqlflow_models.my_xgboost_model;
"""

    def test_antxgboost(self):
        ipython.run_cell_magic("sqlflow", "", self.train_statement)
        ipython.run_cell_magic("sqlflow", "", self.pred_statement)


if __name__ == "__main__":
    unittest.main()
