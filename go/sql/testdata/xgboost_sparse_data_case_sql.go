// Copyright 2020 The SQLFlow Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testdata

import (
	"fmt"
)

const insertData = `
("1:0.5 3:-1.5 5:6.7", 0), ("3:-1.3 10:0.6 2:-3.4", 1), ("5:10.2 3:7.4", 2)`

// XGBoostSparseDataCaseSQL is the data preparation SQL statements
// for sparse data tests
var XGBoostSparseDataCaseSQL = fmt.Sprintf(`
DROP DATABASE IF EXISTS %[1]s;
CREATE DATABASE %[1]s;

CREATE TABLE IF NOT EXISTS %[1]s.%[2]s(
    c1 VARCHAR(255), 
    label_col FLOAT
);

INSERT INTO %[1]s.%[2]s VALUES %s;
`, "xgboost_sparse_data_test_db", "xgboost_sparse_data_train", insertData)

// XGBoostHiveSparseDataCaseSQL is the data preparation SQL statements
// for sparse data tests
var XGBoostHiveSparseDataCaseSQL = fmt.Sprintf(`
DROP DATABASE IF EXISTS %[1]s CASCADE;
CREATE DATABASE %[1]s;

CREATE TABLE IF NOT EXISTS %[1]s.%[2]s(
    c1 VARCHAR(255), 
    label_col FLOAT
);

INSERT INTO %[1]s.%[2]s VALUES %s;
`, "xgboost_sparse_data_test_db", "xgboost_sparse_data_train", insertData)

// XGBoostMaxComputeSparseDataCaseSQL is the data preparation SQL
// statements for sparse data tests in MaxCompute
var XGBoostMaxComputeSparseDataCaseSQL = `
DROP TABLE IF EXISTS %[1]s.xgboost_sparse_data_train;

CREATE TABLE IF NOT EXISTS %[1]s.xgboost_sparse_data_train(
    c1 STRING, 
    label_col DOUBLE
);

INSERT INTO %[1]s.xgboost_sparse_data_train VALUES` + insertData + `;`
