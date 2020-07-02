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

import "fmt"

// IrisMySQL returns a MySQL program that creates and popularizes some tables in db.
func IrisMySQL(db string) string {
	return createDatabase(db) +
		recreateTable(db+".train", irisSchema) +
		insertData(db+".train", irisTrainData) +
		recreateTable(db+".test", irisSchema) +
		insertData(db+".test", irisTestData) +
		mergeIrisFeatures(db+".train", db+".train_dense") +
		mergeIrisFeatures(db+".test", db+".test_dense") +
		recreateTable(db+".iris_empty", irisSchema)
}

// IrisHive returns a Hive program that creates and popularizes some tables in db.
func IrisHive(db string) string {
	return createDatabase(db) +
		recreateTable(db+".train", irisSchema) +
		insertDataHive(db+".train", irisTrainData) +
		recreateTable(db+".test", irisSchema) +
		insertDataHive(db+".test", irisTestData) +
		recreateTable("iris.iris_empty", irisSchema)
}

// IrisMaxCompute returns a MaxCompute program that creates and popularizes some tables in db.
func IrisMaxCompute(proj string) string {
	return recreateTable(proj+".sqlflow_test_iris_train", irisSchemaMaxCompute) +
		insertData(proj+".sqlflow_test_iris_train", irisTrainData) +
		recreateTable(proj+".sqlflow_test_iris_test", irisSchemaMaxCompute) +
		insertData(proj+".sqlflow_test_iris_test", irisTestData) +
		recreateTable(proj+".sqlflow_test_iris_empty", irisSchemaMaxCompute)
}

func mergeIrisFeatures(from, to string) string {
	return dropTableIfExists(to) +
		fmt.Sprintf(`CREATE TABLE %s AS
  (SELECT CONCAT_WS(",", sepal_length, sepal_width, petal_length, petal_width)
          AS dense,
          class
   FROM   %s);
`, to, from)
}

const (
	irisSchema = `
       sepal_length float,
       sepal_width  float,
       petal_length float,
       petal_width  float,
       class int`

	irisSchemaMaxCompute = `
       sepal_length DOUBLE,
       sepal_width  DOUBLE,
       petal_length DOUBLE,
       petal_width  DOUBLE,
       class BIGINT`

	irisTestData = `
(6.3,2.7,4.9,1.8,2),
(5.7,2.8,4.1,1.3,1),
(5.0,3.0,1.6,0.2,0),
(6.3,3.3,6.0,2.5,2),
(5.0,3.5,1.6,0.6,0),
(5.5,2.6,4.4,1.2,1),
(5.7,3.0,4.2,1.2,1),
(4.4,2.9,1.4,0.2,0),
(4.8,3.0,1.4,0.1,0),
(5.5,2.4,3.7,1.0,1)`

	irisTrainData = `
(6.4,2.8,5.6,2.2,2),
(5.0,2.3,3.3,1.0,1),
(4.9,2.5,4.5,1.7,2),
(4.9,3.1,1.5,0.1,0),
(5.7,3.8,1.7,0.3,0),
(4.4,3.2,1.3,0.2,0),
(5.4,3.4,1.5,0.4,0),
(6.9,3.1,5.1,2.3,2),
(6.7,3.1,4.4,1.4,1),
(5.1,3.7,1.5,0.4,0),
(5.2,2.7,3.9,1.4,1),
(6.9,3.1,4.9,1.5,1),
(5.8,4.0,1.2,0.2,0),
(5.4,3.9,1.7,0.4,0),
(7.7,3.8,6.7,2.2,2),
(6.3,3.3,4.7,1.6,1),
(6.8,3.2,5.9,2.3,2),
(7.6,3.0,6.6,2.1,2),
(6.4,3.2,5.3,2.3,2),
(5.7,4.4,1.5,0.4,0),
(6.7,3.3,5.7,2.1,2),
(6.4,2.8,5.6,2.1,2),
(5.4,3.9,1.3,0.4,0),
(6.1,2.6,5.6,1.4,2),
(7.2,3.0,5.8,1.6,2),
(5.2,3.5,1.5,0.2,0),
(5.8,2.6,4.0,1.2,1),
(5.9,3.0,5.1,1.8,2),
(5.4,3.0,4.5,1.5,1),
(6.7,3.0,5.0,1.7,1),
(6.3,2.3,4.4,1.3,1),
(5.1,2.5,3.0,1.1,1),
(6.4,3.2,4.5,1.5,1),
(6.8,3.0,5.5,2.1,2),
(6.2,2.8,4.8,1.8,2),
(6.9,3.2,5.7,2.3,2),
(6.5,3.2,5.1,2.0,2),
(5.8,2.8,5.1,2.4,2),
(5.1,3.8,1.5,0.3,0),
(4.8,3.0,1.4,0.3,0),
(7.9,3.8,6.4,2.0,2),
(5.8,2.7,5.1,1.9,2),
(6.7,3.0,5.2,2.3,2),
(5.1,3.8,1.9,0.4,0),
(4.7,3.2,1.6,0.2,0),
(6.0,2.2,5.0,1.5,2),
(4.8,3.4,1.6,0.2,0),
(7.7,2.6,6.9,2.3,2),
(4.6,3.6,1.0,0.2,0),
(7.2,3.2,6.0,1.8,2),
(5.0,3.3,1.4,0.2,0),
(6.6,3.0,4.4,1.4,1),
(6.1,2.8,4.0,1.3,1),
(5.0,3.2,1.2,0.2,0),
(7.0,3.2,4.7,1.4,1),
(6.0,3.0,4.8,1.8,2),
(7.4,2.8,6.1,1.9,2),
(5.8,2.7,5.1,1.9,2),
(6.2,3.4,5.4,2.3,2),
(5.0,2.0,3.5,1.0,1),
(5.6,2.5,3.9,1.1,1),
(6.7,3.1,5.6,2.4,2),
(6.3,2.5,5.0,1.9,2),
(6.4,3.1,5.5,1.8,2),
(6.2,2.2,4.5,1.5,1),
(7.3,2.9,6.3,1.8,2),
(4.4,3.0,1.3,0.2,0),
(7.2,3.6,6.1,2.5,2),
(6.5,3.0,5.5,1.8,2),
(5.0,3.4,1.5,0.2,0),
(4.7,3.2,1.3,0.2,0),
(6.6,2.9,4.6,1.3,1),
(5.5,3.5,1.3,0.2,0),
(7.7,3.0,6.1,2.3,2),
(6.1,3.0,4.9,1.8,2),
(4.9,3.1,1.5,0.1,0),
(5.5,2.4,3.8,1.1,1),
(5.7,2.9,4.2,1.3,1),
(6.0,2.9,4.5,1.5,1),
(6.4,2.7,5.3,1.9,2),
(5.4,3.7,1.5,0.2,0),
(6.1,2.9,4.7,1.4,1),
(6.5,2.8,4.6,1.5,1),
(5.6,2.7,4.2,1.3,1),
(6.3,3.4,5.6,2.4,2),
(4.9,3.1,1.5,0.1,0),
(6.8,2.8,4.8,1.4,1),
(5.7,2.8,4.5,1.3,1),
(6.0,2.7,5.1,1.6,1),
(5.0,3.5,1.3,0.3,0),
(6.5,3.0,5.2,2.0,2),
(6.1,2.8,4.7,1.2,1),
(5.1,3.5,1.4,0.3,0),
(4.6,3.1,1.5,0.2,0),
(6.5,3.0,5.8,2.2,2),
(4.6,3.4,1.4,0.3,0),
(4.6,3.2,1.4,0.2,0),
(7.7,2.8,6.7,2.0,2),
(5.9,3.2,4.8,1.8,1),
(5.1,3.8,1.6,0.2,0),
(4.9,3.0,1.4,0.2,0),
(4.9,2.4,3.3,1.0,1),
(4.5,2.3,1.3,0.3,0),
(5.8,2.7,4.1,1.0,1),
(5.0,3.4,1.6,0.4,0),
(5.2,3.4,1.4,0.2,0),
(5.3,3.7,1.5,0.2,0),
(5.0,3.6,1.4,0.2,0),
(5.6,2.9,3.6,1.3,1),
(4.8,3.1,1.6,0.2,0)`
)
