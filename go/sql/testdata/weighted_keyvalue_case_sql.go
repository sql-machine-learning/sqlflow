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

// Sample data for testing tf.weighted_category_column feature parsing

const weightedKeyValueInsertData = `
("a:0.1,b:0.5,c:0.9,d:0.8", 0),
("c:0.2,d:0.1", 1),
("a:0.1,c:0.8,d:0.9", 0)`

const weightedKeyValueInsertDataInt = `
("1:0.1,2:0.5,3:0.9,4:0.8", 0),
("3:0.2,4:0.1", 1),
("1:0.1,3:0.8,4:0.9", 0)`

// WeightedKeyValueCaseSQL is for mysql.
var WeightedKeyValueCaseSQL = `
DROP DATABASE IF EXISTS %[1]s;
CREATE DATABASE %[1]s;

CREATE TABLE IF NOT EXISTS %[1]s.weighted_key_value_train(
    feature VARCHAR(255), 
    label_col INT
);

INSERT INTO %[1]s.weighted_key_value_train VALUES ` + weightedKeyValueInsertData + `;` + `

CREATE TABLE IF NOT EXISTS %[1]s.weighted_key_value_train_int(
    feature VARCHAR(255), 
    label_col INT
);

INSERT INTO %[1]s.weighted_key_value_train_int VALUES ` + weightedKeyValueInsertDataInt + `;`

// WeightedKeyValueCaseSQLHive is for hive.
var WeightedKeyValueCaseSQLHive = `
DROP DATABASE IF EXISTS %[1]s CASCADE;
CREATE DATABASE %[1]s;

CREATE TABLE IF NOT EXISTS %[1]s.weighted_key_value_train(
    feature VARCHAR(255), 
    label_col INT
);

INSERT INTO %[1]s.weighted_key_value_train VALUES ` + weightedKeyValueInsertData + `;` + `

CREATE TABLE IF NOT EXISTS %[1]s.weighted_key_value_train_int(
    feature VARCHAR(255), 
    label_col INT
);

INSERT INTO %[1]s.weighted_key_value_train_int VALUES ` + weightedKeyValueInsertDataInt + `;`

// WeightedKeyValueCaseSQLMaxCompute is for maxcompute.
var WeightedKeyValueCaseSQLMaxCompute = `
DROP TABLE IF EXISTS %[1]s.weighted_key_value_train;

CREATE TABLE IF NOT EXISTS %[1]s.weighted_key_value_train(
    feature STRING, 
    label_col INT
);

INSERT INTO %[1]s.weighted_key_value_train VALUES ` + weightedKeyValueInsertData + `;` + `

CREATE TABLE IF NOT EXISTS %[1]s.weighted_key_value_train_int(
    feature STRING, 
    label_col INT
);

INSERT INTO %[1]s.weighted_key_value_train_int VALUES ` + weightedKeyValueInsertDataInt + `;`
