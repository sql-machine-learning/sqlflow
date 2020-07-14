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

package optimize

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"reflect"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/ir"
	"strings"
	"testing"
)

func replaceResultValueName(stmt *ir.OptimizeStmt, newName string) {
	oldName := stmt.ResultValueName
	stmt.ResultValueName = newName

	for i, v := range stmt.Variables {
		if strings.EqualFold(v, oldName) {
			stmt.Variables[i] = newName
		}
	}

	for i, token := range stmt.Objective.ExpressionTokens {
		if strings.EqualFold(token, oldName) {
			stmt.Objective.ExpressionTokens[i] = newName
		}
	}

	for _, c := range stmt.Constraints {
		for i, token := range c.ExpressionTokens {
			if strings.EqualFold(token, oldName) {
				c.ExpressionTokens[i] = newName
			}
		}

		if strings.EqualFold(c.GroupBy, oldName) {
			c.GroupBy = newName
		}
	}
}

func generateTestOptimizeStmt(selectStmt string, variables []string, resultValueName string,
	objective []string, constraints [][]string, constraintGroupBy []string) *ir.OptimizeStmt {
	constraintExprs := make([]*ir.OptimizeExpr, len(constraints))
	for i := range constraints {
		groupBy := ""
		if constraintGroupBy != nil {
			groupBy = constraintGroupBy[i]
		}
		constraintExprs[i] = &ir.OptimizeExpr{
			ExpressionTokens: constraints[i],
			GroupBy:          groupBy,
		}
	}

	return &ir.OptimizeStmt{
		Select:          selectStmt,
		Variables:       variables,
		ResultValueName: resultValueName,
		Objective:       ir.OptimizeExpr{ExpressionTokens: objective},
		Constraints:     constraintExprs,
	}
}

func generateTestExpressions(stmt *ir.OptimizeStmt, t *testing.T) (string, []string) {
	testDBType := os.Getenv("SQLFLOW_TEST_DB")
	if testDBType != "mysql" {
		t.Skip("skip when SQLFLOW_TEST_DB is not mysql")
	}

	db := database.GetTestingDBSingleton()
	dbName := "optimize_test_db"

	tmpTableName := fmt.Sprintf("%s.optimize_optflow_test_table", dbName)
	createTableSQL := fmt.Sprintf(`CREATE TABLE %s AS %s`, tmpTableName, stmt.Select)

	_, err := db.Exec(createTableSQL)
	assert.NoError(t, err)
	defer db.Exec(fmt.Sprintf("DROP TABLE %s;", tmpTableName))

	objective, constraints, err := generateOptFlowObjectiveAndConstraintExpressions(stmt, db, tmpTableName)
	assert.NoError(t, err)
	return objective, constraints
}

func TestOptimizeExpressionGenerationWithoutGroupBy(t *testing.T) {
	objective := []string{
		"SUM", "(", "(", "price", "-", "materials_cost", "-", "other_cost",
		")", "*", "product", ")",
	}

	constraints := [][]string{
		{"SUM", "(", "finishing", "*", "product", ")", "<=", "100"},
		{"SUM", "(", "carpentry", "*", "product", ")", "<=", "80"},
		{"product", "<=", "max_num"},
	}

	stmt := generateTestOptimizeStmt(`SELECT * FROM optimize_test_db.woodcarving`,
		[]string{"product"}, "product", objective, constraints, nil)

	objExpr1, cExprs1 := generateTestExpressions(stmt, t)
	assert.Equal(t, `sum([(@input["price"][i_0]-@input["materials_cost"][i_0]-@input["other_cost"][i_0])*@X[i_0] for i_0 in @X])`, objExpr1)
	assert.True(t, reflect.DeepEqual(
		cExprs1, []string{
			`sum([@input["finishing"][i_0]*@X[i_0] for i_0 in @X])<=100`,
			`sum([@input["carpentry"][i_0]*@X[i_0] for i_0 in @X])<=80`,
			`for i in @X: @X[i]<=@input["max_num"][i]`,
		}))

	replaceResultValueName(stmt, "my_product_name")
	objExpr2, cExprs2 := generateTestExpressions(stmt, t)
	assert.Equal(t, objExpr1, objExpr2)
	assert.True(t, reflect.DeepEqual(cExprs1, cExprs2))

	replaceResultValueName(stmt, "product")
	objExpr3, cExprs3 := generateTestExpressions(stmt, t)
	assert.Equal(t, objExpr1, objExpr3)
	assert.True(t, reflect.DeepEqual(cExprs1, cExprs3))

	replaceResultValueName(stmt, "product")
	stmt.Objective.ExpressionTokens = []string{
		"SUM", "(", "finishing", "*", "product", "+", "SUM", "(",
		"product", ")", ")", "<=", "100",
	}
	objExpr4, cExprs4 := generateTestExpressions(stmt, t)
	assert.Equal(t, `sum([@input["finishing"][i_0]*@X[i_0]+sum([@X[i_1] for i_1 in @X]) for i_0 in @X])<=100`, objExpr4)
	assert.True(t, reflect.DeepEqual(cExprs1, cExprs4))

	replaceResultValueName(stmt, "my_product_name")
	objExpr5, cExprs5 := generateTestExpressions(stmt, t)
	assert.Equal(t, objExpr4, objExpr5)
	assert.True(t, reflect.DeepEqual(cExprs1, cExprs5))
}

func TestOptimizeExpressionGenerationWithGroupBy(t *testing.T) {
	objective := []string{
		"SUM", "(", "distance", "*", "shipment", "*", "90", "/", "1000",
		")",
	}

	constraints := [][]string{
		{"SUM", "(", "shipment", ")", "<=", "capacity"},
		{"SUM", "(", "shipment", ")", ">=", "demand"},
		{"shipment", "*", "100", ">=", "demand"},
	}

	groupBys := []string{"plants", "markets", ""}

	stmt := generateTestOptimizeStmt(`SELECT 
		t.plants AS plants, 
		t.markets AS markets, 
		t.distance AS distance, 
		p.capacity AS capacity, 
		m.demand AS demand FROM optimize_test_db.transportation_table AS t
    LEFT JOIN optimize_test_db.plants_table AS p ON t.plants = p.plants
    LEFT JOIN optimize_test_db.markets_table AS m ON t.markets = m.markets ORDER BY plants, markets`,
		[]string{"plants", "markets"}, "shipment", objective, constraints, groupBys)

	objExpr1, cExprs1 := generateTestExpressions(stmt, t)
	assert.Equal(t, `sum([@input["distance"][i_0]*@X[i_0]*90/1000 for i_0 in @X])`, objExpr1)
	assert.True(t, reflect.DeepEqual(
		cExprs1, []string{
			`for value, index in zip(*__import__("numpy").unique(@input["plants"].to_numpy(), return_index=True)): sum([@X[i_0] for i_0 in __import__("numpy").where(@input["plants"] == value)[0].tolist()])<=@input["capacity"][index]`,
			`for value, index in zip(*__import__("numpy").unique(@input["markets"].to_numpy(), return_index=True)): sum([@X[i_0] for i_0 in __import__("numpy").where(@input["markets"] == value)[0].tolist()])>=@input["demand"][index]`,
			`for i in @X: @X[i]*100>=@input["demand"][i]`,
		}))
}
