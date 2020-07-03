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

// StandardJoinTest some simple data for testing standard SQL join which
// should be bypass to the SQL engine.
var StandardJoinTest = `CREATE DATABASE IF NOT EXISTS standard_join_test;
DROP TABLE IF EXISTS standard_join_test.user_fea1;
CREATE TABLE standard_join_test.user_fea1 (
       user_id int,
       fea1 TEXT);

DROP TABLE IF EXISTS standard_join_test.user_fea2;
CREATE TABLE standard_join_test.user_fea2 (
       user_id int,
       fea2 TEXT);

INSERT INTO standard_join_test.user_fea1 VALUES
(1,"1,2,1,39,0,0,0,23,0"),
(2,"2,82,3,3,1,1,0,0,0"),
(3,"3,82,3,3,1,1,0,0,0"),
(4,"4,82,3,3,1,1,0,0,0"),
(5,"5,82,3,3,1,1,0,0,0");

INSERT INTO standard_join_test.user_fea2 VALUES
(1,"11,2,1,39,0,0,0,23,0"),
(2,"22,82,3,3,1,1,0,0,0"),
(3,"33,82,3,3,1,1,0,0,0"),
(4,"44,82,3,3,1,1,0,0,0"),
(5,"55,82,3,3,1,1,0,0,0");`
