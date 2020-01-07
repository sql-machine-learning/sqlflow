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

package verifier

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/parser"
)

func parseOneStatement(statement string) (*parser.SQLFlowSelectStmt, error) {
	sqls, err := parser.Parse("mysql", statement)
	if err != nil {
		return nil, err
	}
	if len(sqls) != 1 {
		return nil, fmt.Errorf(`len(sqls) != 1, actual %d`, len(sqls))
	}

	return sqls[0].SQLFlowSelectStmt, nil
}

func TestVerify(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST_DB") == "hive" {
		t.Skip("in Hive, db_name.table_name.field_name will raise error, because . operator is only supported on struct or list of struct types")
	}

	a := assert.New(t)

	{
		fts, e := Verify(`SELECT * FROM churn.train LIMIT 10;`, database.GetTestingDBSingleton())
		a.NoError(e)
		a.Equal(21, len(fts))
	}

	{
		fts, e := Verify(`SELECT Churn, churn.train.Partner,TotalCharges FROM churn.train LIMIT 10;`, database.GetTestingDBSingleton())
		a.NoError(e)
		a.Equal(3, len(fts))

		typ, ok := fts.Get("churn")
		a.True(ok)
		a.Contains([]string{"VARCHAR(255)", "VARCHAR"}, typ)

		typ, ok = fts.Get("partner")
		a.True(ok)
		a.Contains([]string{"VARCHAR(255)", "VARCHAR"}, typ)

		typ, ok = fts.Get("totalcharges")
		a.True(ok)
		a.Equal("FLOAT", typ)
	}
}

func TestVerifyColumnNameAndType(t *testing.T) {
	a := assert.New(t)
	trainParse, e := parseOneStatement(`SELECT gender, tenure, TotalCharges
FROM churn.train LIMIT 10
TO TRAIN DNNClassifier
WITH
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN gender, tenure, totalcharges
LABEL class
INTO sqlflow_models.my_dnn_model;`)
	a.NoError(e)

	predParse, e := parseOneStatement(`SELECT gender, tenure, TotalCharges
FROM churn.train LIMIT 10
TO PREDICT iris.predict.class
USING sqlflow_models.my_dnn_model;`)
	a.NoError(e)
	a.NoError(VerifyColumnNameAndType(trainParse, predParse, database.GetTestingDBSingleton()))

	predParse, e = parseOneStatement(`SELECT gender, tenure
FROM churn.train LIMIT 10
TO PREDICT iris.predict.class
USING sqlflow_models.my_dnn_model;`)
	a.NoError(e)
	a.EqualError(VerifyColumnNameAndType(trainParse, predParse, database.GetTestingDBSingleton()),
		"predFields doesn't contain column totalcharges")
}

func TestDescribeEmptyTables(t *testing.T) {
	a := assert.New(t)
	_, e := Verify(`SELECT * FROM iris.iris_empty LIMIT 10;`, database.GetTestingDBSingleton())
	a.EqualError(e, `query SELECT * FROM iris.iris_empty LIMIT 10; gives 0 row`)
}
