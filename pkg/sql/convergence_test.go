// Copyright 2019 The SQLFlow Authors. All rights reserved.
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

package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/pkg/database"
)

// We train a DNNClassifier on five data points and let it reaches 100 percent accuracy.
// Then we do a prediction on the same data points. We expect the prediction accuracy
// also be 100 percent.
func TestConvergenceAndAccuracy(t *testing.T) {
	testDB := database.GetTestingDBSingleton()
	if testDB.DriverName != "mysql" {
		t.Skip("only run convergence test with MySQL")
	}
	a := assert.New(t)
	modelDir := ""
	a.NotPanics(func() {
		stream := RunSQLTestCase(`
SELECT * FROM sanity_check.train
TO TRAIN DNNClassifier
WITH
	model.n_classes = 5,
	model.hidden_units = [42, 13],
	model.optimizer = "Adam",
	train.epoch = 100,
	validation.select="SELECT * FROM sanity_check.train"
LABEL class
INTO sqlflow_models.my_dnn_model;
`, modelDir, database.GetSessionFromTestingDB())
		a.True(goodStream(stream.ReadAll()))
	})
	a.NotPanics(func() {
		stream := RunSQLTestCase(`
SELECT * FROM sanity_check.train
TO PREDICT sanity_check.predict.class
USING sqlflow_models.my_dnn_model;
`, modelDir, database.GetSessionFromTestingDB())
		a.True(goodStream(stream.ReadAll()))
	})
	a.NotPanics(func() {
		rows, err := testDB.Query("select * from sanity_check.predict order by class")
		a.NoError(err)
		actualRows := [][6]int{}
		for rows.Next() {
			var c0, c1, c2, c3, c4, class int
			err := rows.Scan(&c0, &c1, &c2, &c3, &c4, &class)
			a.NoError(err)
			actualRows = append(actualRows, [6]int{c0, c1, c2, c3, c4, class})
		}
		expectedRows := [][6]int{
			{1, 0, 0, 0, 0, 0},
			{0, 1, 0, 0, 0, 1},
			{0, 0, 1, 0, 0, 2},
			{0, 0, 0, 1, 0, 3},
			{0, 0, 0, 0, 1, 4},
		}
		a.Equal(len(expectedRows), len(actualRows))
		for i := 0; i < len(expectedRows); i++ {
			for j := 0; j < 6; j++ {
				a.Equal(expectedRows[i][j], actualRows[i][j])
			}
		}
	})
}
