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

// SanityCheckSQL provides data for sanity checking the trained model is learning something
// We should expect the train accuracy to be 1.0 and predict accuracy to be 1.0.
var SanityCheckSQL = `CREATE DATABASE IF NOT EXISTS sanity_check;
DROP TABLE IF EXISTS sanity_check.train;
CREATE TABLE sanity_check.train (
	c0 int,
	c1 int,
	c2 int,
	c3 int,
	c4 int,
	class int);
INSERT INTO sanity_check.train VALUES
(1, 0, 0, 0, 0, 0),
(0, 1, 0, 0, 0, 1),
(0, 0, 1, 0, 0, 2),
(0, 0, 0, 1, 0, 3),
(0, 0, 0, 0, 1, 4);

DROP TABLE IF EXISTS sanity_check.test;
CREATE TABLE sanity_check.test (
	id int,
	c0 int,
	c1 int,
	c2 int,
	c3 int,
	c4 int,
	class int);
INSERT INTO sanity_check.test VALUES
(0, 1, 0, 0, 0, 0, 0),
(1, 0, 1, 0, 0, 0, 1),
(2, 0, 0, 1, 0, 0, 2),
(3, 0, 0, 0, 1, 0, 3),
(4, 0, 0, 0, 0, 1, 4);
`
