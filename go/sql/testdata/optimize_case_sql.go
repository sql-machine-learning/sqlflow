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

// OptimizeCaseSQL is the data preparation SQL statements for optimize test cases.
const OptimizeCaseSQL = `CREATE DATABASE IF NOT EXISTS optimize_test_db;

DROP TABLE IF EXISTS optimize_test_db.woodcarving;
CREATE TABLE optimize_test_db.woodcarving (
    product VARCHAR(255),
    price BIGINT,
    materials_cost BIGINT,
    other_cost BIGINT,
    finishing BIGINT,
    carpentry BIGINT,
    max_num BIGINT
);
INSERT INTO optimize_test_db.woodcarving VALUES('soldier', 27, 10, 14, 2, 1, 40),
('train', 21, 9, 10, 1, 1, 10000);

DROP TABLE IF EXISTS optimize_test_db.plants_table;
CREATE TABLE optimize_test_db.plants_table (
    plants VARCHAR(255),
    capacity BIGINT
);
INSERT INTO optimize_test_db.plants_table VALUES('plantA', 100), ('plantB', 90);

DROP TABLE IF EXISTS optimize_test_db.markets_table;
CREATE TABLE optimize_test_db.markets_table (
    markets VARCHAR(255),
    demand BIGINT
);
INSERT INTO optimize_test_db.markets_table VALUES('marketA', 130), ('marketB', 60);

DROP TABLE IF EXISTS optimize_test_db.transportation_table;
CREATE TABLE optimize_test_db.transportation_table (
    plants VARCHAR(255),
    markets VARCHAR(255),
    distance BIGINT
);
INSERT INTO optimize_test_db.transportation_table VALUES('plantA', 'marketA', 140), 
('plantA', 'marketB', 210), ('plantB', 'marketA', 300),
('plantB', 'marketB', 90);
`
