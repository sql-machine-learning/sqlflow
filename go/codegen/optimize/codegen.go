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
	"bytes"
	"encoding/json"
	"fmt"
	"sqlflow.org/sqlflow/go/attribute"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
	"sqlflow.org/sqlflow/go/verifier"
	"strings"
	"text/template"
)

var attributeDictionary = attribute.Dictionary{}.
	Bool("data.enable_slice", false, "Whether to enable data slicing", nil).
	Int("data.batch_size", -1, "Batch size when training", nil).
	Int("worker.num", 1, "Worker number", attribute.IntLowerBoundChecker(1, true)).
	Int("worker.core", 8, "Worker core number", attribute.IntLowerBoundChecker(1, true)).
	Int("worker.memory", 4096, "Worker memory", attribute.IntLowerBoundChecker(1, true)).
	Unknown("solver.*", nil, "Solver options", nil)

// InitializeAttributes initialize attributes in optimize clause IR
func InitializeAttributes(stmt *ir.OptimizeStmt) error {
	attributeDictionary.ExportDefaults(stmt.Attributes)
	err := attributeDictionary.Validate(stmt.Attributes)
	return err
}

func generateOptimizeAttributeJSONString(attrs map[string]interface{}) (string, error) {
	const (
		dataAttrPrefix   = "data."
		solverAttrPrefix = "solver."
		workerAttrPrefix = "worker."
	)

	parsedAttrs := make(map[string]map[string]interface{})
	for k, v := range attrs {
		prefix := ""
		if strings.HasPrefix(k, dataAttrPrefix) {
			prefix = dataAttrPrefix
		} else if strings.HasPrefix(k, solverAttrPrefix) {
			prefix = solverAttrPrefix
		} else if strings.HasPrefix(k, workerAttrPrefix) {
			prefix = workerAttrPrefix
		} else {
			return "", fmt.Errorf("unrecognized attribute %s", k)
		}

		k = k[len(prefix):]
		prefixKey := prefix[0 : len(prefix)-1]
		if _, ok := parsedAttrs[prefixKey]; !ok {
			parsedAttrs[prefixKey] = make(map[string]interface{})
		}
		parsedAttrs[prefixKey][k] = v
	}

	parsedAttrJSON, err := json.Marshal(parsedAttrs)
	if err != nil {
		return "", err
	}
	return string(parsedAttrJSON), nil
}

func generateGroupByRangeAndIndexStr(groupBy string, dataStr string) (string, string, string) {
	const (
		indexStr = `index`
		numpyStr = `__import__("numpy")`
	)
	groupByDataStr := fmt.Sprintf(`%s["%s"]`, dataStr, groupBy)
	outerRangeStr := fmt.Sprintf(`for value, %s in zip(*%s.unique(%s.to_numpy(), return_index=True))`, indexStr, numpyStr, groupByDataStr)
	innerRangeStr := fmt.Sprintf(`%s.where(%s == value)[0].tolist()`, numpyStr, groupByDataStr)
	return outerRangeStr, innerRangeStr, indexStr
}

// tryConvertToAggregationFunction tries to convert the token to an aggregation function
// name. If the token is not an aggregation function name, it returns "".
func tryConvertToAggregationFunction(token string) string {
	// the key of aggregationFunctions is the aggregation function name of SQL.
	// the value of aggregationFunctions is the aggregation function name of Python.
	var aggregationFunctions = map[string]string{"SUM": "sum"}
	return aggregationFunctions[strings.ToUpper(token)]
}

// updateOptimizeStmtByTableColumnNames updates the OptimizeStmt by the column names
// returned by selectStmt. This function returns the column names.
func updateOptimizeStmtByTableColumnNames(stmt *ir.OptimizeStmt, db *database.DB, selectStmt string) ([]string, error) {
	rows, err := verifier.FetchNSamples(db, selectStmt, 1)
	if err != nil {
		return nil, err
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	columnNames := make([]string, len(columnTypes))
	for i, c := range columnTypes {
		nameSplit := strings.Split(c.Name(), ".")
		columnNames[i] = nameSplit[len(nameSplit)-1]
	}

	// toColumnNameOrReturnItself is used to ignore the letter cases
	// of the column name in the SQL statement. We try to replace the
	// column name in the SQL statement with the column name queried
	// from the database. In this way, we can write less
	// strings.EqualFold() or a.lower() == b.lower() codes in the
	// following function calls.
	toColumnNameOrReturnItself := func(name string) (string, bool) {
		for _, n := range columnNames {
			if strings.EqualFold(n, name) {
				return n, true
			}
		}
		return name, false
	}

	// update variable names in the SQL statement by the queried table
	// names from database.
	for i, v := range stmt.Variables {
		newVarName, ok := toColumnNameOrReturnItself(v)
		if !ok {
			return nil, fmt.Errorf("cannot find column %s in table", v)
		}
		stmt.Variables[i] = newVarName
	}

	resultValueName, _ := toColumnNameOrReturnItself(stmt.ResultValueName)
	stmt.ResultValueName = resultValueName
	if len(stmt.Variables) == 1 && stmt.Variables[0] == stmt.ResultValueName {
		stmt.ResultValueName += "_value"
	}

	for i, token := range stmt.Objective.ExpressionTokens {
		token, _ = toColumnNameOrReturnItself(token)
		if token == resultValueName {
			// If the user writes "WITH variables="amount(product)",
			// no replacement would be done.
			// If the user writes "WITH variables="product", the "product"
			// in SELECT clause would be replaced with "product_value".
			token = stmt.ResultValueName
		}
		stmt.Objective.ExpressionTokens[i] = token
	}

	for _, c := range stmt.Constraints {
		for i, token := range c.ExpressionTokens {
			// same logic as how the objective token updates.
			token, _ = toColumnNameOrReturnItself(token)
			if token == resultValueName {
				token = stmt.ResultValueName
			}
			c.ExpressionTokens[i] = token
		}

		if c.GroupBy != "" {
			newGroupByName, ok := toColumnNameOrReturnItself(c.GroupBy)
			if !ok {
				return nil, fmt.Errorf("cannot find GROUP BY column %s in table", c.GroupBy)
			}
			c.GroupBy = newGroupByName
		}
	}
	return columnNames, nil
}

// findPrevNonBlankToken tries to find the first previous
// non-blank token from index i (including i). If there is
// no non-blank token, it returns -1.
func findPrevNonBlankToken(tokens []string, i int) int {
	if i < 0 || i >= len(tokens) {
		return -1
	}

	for ; i >= 0; i-- {
		if strings.TrimSpace(tokens[i]) != "" {
			return i
		}
	}
	return -1
}

// findMatchedAggregationFunctionBrackets finds the matched brackets of
// the aggregation functions in the tokens.
// The returned values are (leftBracketIndices, rightBracketIndices, nextIdx),
// where nextIdx is the index for the next bracket searching.
func findMatchedAggregationFunctionBrackets(tokens []string, startIdx int) ([]int, []int, int, error) {
	leftBracketIndices := make([]int, 0)
	rightBracketIndices := make([]int, 0)

	leftBracketNum := 0
	for ; startIdx < len(tokens); startIdx++ {
		if tokens[startIdx] == "(" {
			leftBracketIndices = append(leftBracketIndices, startIdx)
			// Put -1 in rightBracketIndices first, and it would be
			// filled with other values if ")" is found
			rightBracketIndices = append(rightBracketIndices, -1)
			leftBracketNum++
		} else if tokens[startIdx] == ")" {
			if leftBracketNum <= 0 {
				return nil, nil, 0, fmt.Errorf("bracket not match")
			}
			leftBracketNum--
			rightBracketIndices[leftBracketNum] = startIdx
			if leftBracketNum == 0 {
				startIdx++ // make startIdx be the next index
				break
			}
		}
	}

	if leftBracketNum != 0 {
		return nil, nil, 0, fmt.Errorf("bracket not match")
	}

	aggFuncLeftBracketIndices := make([]int, 0)
	aggFuncRightBracketIndices := make([]int, 0)

	for i, idx := range leftBracketIndices {
		prevIdx := findPrevNonBlankToken(tokens, idx-1)
		// not all brackets belong to an aggregation function.
		// for example: SUM((a - b) * c), the bracket in (a - b) should be ignored.
		if prevIdx >= 0 && tryConvertToAggregationFunction(tokens[prevIdx]) != "" {
			aggFuncLeftBracketIndices = append(aggFuncLeftBracketIndices, leftBracketIndices[i])
			aggFuncRightBracketIndices = append(aggFuncRightBracketIndices, rightBracketIndices[i])
		}
	}

	if startIdx > len(tokens) {
		startIdx = len(tokens)
	}

	return aggFuncLeftBracketIndices, aggFuncRightBracketIndices, startIdx, nil
}

// generateTokenInNonAggregationExpression generates the proper token in the non-aggregation expression part
func generateTokenInNonAggregationExpression(token string, groupBy string, stmt *ir.OptimizeStmt, columns []string, dataStr string, indexStr string) (string, error) {
	if tryConvertToAggregationFunction(token) != "" {
		return tryConvertToAggregationFunction(token), nil
	}

	if token == stmt.ResultValueName {
		if len(stmt.Variables) == 1 {
			return fmt.Sprintf(`%s["%s"]`, dataStr, stmt.Variables[0]), nil
		}
		return "", fmt.Errorf("invalid expression: result variable %s should not occur in objective or constraint", token)
	}

	// variables should not occur in non-aggregation expression
	for _, v := range stmt.Variables {
		if token == v {
			return "", fmt.Errorf("invalid expression: variable %s should not occur in objective or constraint", v)
		}
	}

	for _, v := range columns {
		if token != v {
			continue
		}

		if groupBy == "" {
			return "", fmt.Errorf("column %s should not occur without GROUP BY", token)
		}

		// return the value of the first row when using GROUP BY
		return fmt.Sprintf(`%s["%s"][%s]`, dataStr, token, indexStr), nil
	}
	return token, nil
}

// generateTokenInNonAggregationExpression generates the proper token in the aggregation expression part
func generateTokenInAggregationExpression(token string,
	stmt *ir.OptimizeStmt,
	columns []string,
	variableStr string, dataStr string, depth int) (string, error) {
	if tryConvertToAggregationFunction(token) != "" {
		return tryConvertToAggregationFunction(token), nil
	}

	if token == stmt.ResultValueName {
		return fmt.Sprintf(`%s[i_%d]`, variableStr, depth), nil
	}

	for _, c := range columns {
		if token == c {
			return fmt.Sprintf(`%s["%s"][i_%d]`, dataStr, token, depth), nil
		}
	}

	return token, nil
}

func getBracketDepth(idx int, leftBracketIndices []int, rightBracketIndices []int) (int, error) {
	depthIdx := -1
	for bracketIdx := 0; bracketIdx < len(leftBracketIndices); bracketIdx++ {
		if idx >= leftBracketIndices[bracketIdx] && idx <= rightBracketIndices[bracketIdx] {
			depthIdx++
		}
	}

	if depthIdx < 0 {
		return 0, fmt.Errorf("cannot find bracket depth")
	}
	return depthIdx, nil
}

// generateNonAggregatedConstraintExpression generates the expression from tokens without aggregation functions.
// Input SQL statement: CONSTRAINT product >= demand , where product is the variable
// Output expression: @X[i] >= @input["demand"][i], if variableStr = "@X" and dataStr = "@input"
func generateNonAggregatedConstraintExpression(tokens []string, stmt *ir.OptimizeStmt, columns []string, variableStr string, dataStr string) (string, string, error) {
	resultTokens := make([]string, 0)

	for _, token := range tokens {
		if token == stmt.ResultValueName {
			resultTokens = append(resultTokens, fmt.Sprintf("%s[i]", variableStr))
			continue
		}

		shouldContinue := false
		for _, c := range columns {
			if token == c {
				resultTokens = append(resultTokens, fmt.Sprintf(`%s["%s"][i]`, dataStr, token))
				shouldContinue = true
				break
			}
		}

		if shouldContinue {
			continue
		}

		resultTokens = append(resultTokens, token)
	}

	rangeStr := fmt.Sprintf("for i in %s", variableStr)
	return strings.Join(resultTokens, ""), rangeStr, nil
}

// generateObjectiveOrAggregatedConstraintExpression generates the expression from tokens with aggregation functions.
// Input SQL statement: TO MAXIMIZE sum((price - cost) * amount)
// Output expression: sum([(@input["price"] - @input["cost"]) * @X[i] for i in @X]), if variableStr = "@X" and dataStr = "@input"
// If there is GROUP BY in the CONSTRAINT clause, indices must be provided.
func generateObjectiveOrAggregatedConstraintExpression(
	tokens []string, groupBy string, stmt *ir.OptimizeStmt, columns []string, variableStr string, dataStr string) (string, string, error) {
	idx := 0
	resultTokens := make([]string, 0)

	indexStr := ""
	outerRangeStr := ""
	innerRangeStr := ""
	if groupBy != "" {
		outerRangeStr, innerRangeStr, indexStr = generateGroupByRangeAndIndexStr(groupBy, dataStr)
	}

	contains := func(slice []int, v int) bool {
		for _, sliceValue := range slice {
			if sliceValue == v {
				return true
			}
		}
		return false
	}

	for idx < len(tokens) {
		leftBracketIndices, rightBracketIndices, nextIdx, err := findMatchedAggregationFunctionBrackets(tokens, idx)
		if err != nil {
			return "", "", err
		}

		leftBracketIdx := nextIdx
		rightBracketIdx := nextIdx
		if len(leftBracketIndices) != 0 {
			leftBracketIdx = leftBracketIndices[0]
			rightBracketIdx = rightBracketIndices[0]
		}

		for ; idx < leftBracketIdx; idx++ {
			token, err := generateTokenInNonAggregationExpression(tokens[idx], groupBy, stmt, columns, dataStr, indexStr)
			if err != nil {
				return "", "", err
			}
			resultTokens = append(resultTokens, token)
		}

		if leftBracketIdx == rightBracketIdx { // only when len(leftBracketIndices) == 0
			continue
		}

		for idx = leftBracketIdx; idx <= rightBracketIdx; idx++ {
			if tokens[idx] == "(" {
				resultTokens = append(resultTokens, tokens[idx])
				if contains(leftBracketIndices, idx) { // left bracket of the SUM(...)
					resultTokens = append(resultTokens, "[")
				}
				continue
			}

			depth, err := getBracketDepth(idx, leftBracketIndices, rightBracketIndices)
			if err != nil {
				return "", "", err
			}

			if tokens[idx] == ")" {
				if contains(rightBracketIndices, idx) { // right bracket of the SUM(...)
					resultTokens = append(resultTokens, " ")
					forRangeStr := ""
					if groupBy == "" {
						forRangeStr = fmt.Sprintf(`for i_%d in %s`, depth, variableStr)
					} else {
						forRangeStr = fmt.Sprintf(`for i_%d in %s`, depth, innerRangeStr)
					}
					resultTokens = append(resultTokens, forRangeStr, "]")
				}
				resultTokens = append(resultTokens, tokens[idx])
				continue
			}

			token, err := generateTokenInAggregationExpression(tokens[idx], stmt, columns, variableStr, dataStr, depth)
			if err != nil {
				return "", "", err
			}
			resultTokens = append(resultTokens, token)
		}

		for idx = rightBracketIdx + 1; idx < nextIdx; idx++ {
			token, err := generateTokenInNonAggregationExpression(tokens[idx], groupBy, stmt, columns, dataStr, indexStr)
			if err != nil {
				return "", "", err
			}
			resultTokens = append(resultTokens, token)
		}
	}

	return strings.Join(resultTokens, ""), outerRangeStr, nil
}

type rangedExpression struct {
	Expr  string
	Range string
}

// generateObjectiveOrConstraintExpressions generates expression of objective or constraint
func generateObjectiveOrConstraintExpressions(tokens []string, groupBy string, stmt *ir.OptimizeStmt, columns []string,
	variableStr string, dataStr string) ([]*rangedExpression, error) {
	hasAggregationFunc := false
	for _, token := range tokens {
		if tryConvertToAggregationFunction(token) != "" {
			hasAggregationFunc = true
			break
		}
	}

	expressions := make([]*rangedExpression, 0)
	if groupBy != "" {
		if !hasAggregationFunc {
			return nil, fmt.Errorf("GROUP BY %s must be used with aggregation functions", groupBy)
		}

		expr, rangeStr, err := generateObjectiveOrAggregatedConstraintExpression(tokens, groupBy, stmt, columns, variableStr, dataStr)
		if err != nil {
			return nil, err
		}
		expressions = append(expressions, &rangedExpression{Expr: expr, Range: rangeStr})
		return expressions, nil
	}

	if hasAggregationFunc {
		expr, rangeStr, err := generateObjectiveOrAggregatedConstraintExpression(tokens, "", stmt, columns, variableStr, dataStr)
		if err != nil {
			return nil, err
		}
		expressions = append(expressions, &rangedExpression{Expr: expr, Range: rangeStr})
	} else {
		expr, rangeStr, err := generateNonAggregatedConstraintExpression(tokens, stmt, columns, variableStr, dataStr)
		if err != nil {
			return nil, err
		}
		expressions = append(expressions, &rangedExpression{Expr: expr, Range: rangeStr})
	}
	return expressions, nil
}

func generateOptFlowObjectiveAndConstraintExpressions(stmt *ir.OptimizeStmt, db *database.DB, tableName string) (string, []string, error) {
	const (
		optFlowVariableStr = "@X"
		optFlowDataStr     = "@input"
	)

	columns, err := updateOptimizeStmtByTableColumnNames(stmt, db, fmt.Sprintf("SELECT * FROM %s", tableName))
	if err != nil {
		return "", nil, err
	}

	objectiveExpr, err := generateObjectiveOrConstraintExpressions(stmt.Objective.ExpressionTokens, "",
		stmt, columns, optFlowVariableStr, optFlowDataStr)
	if err != nil {
		return "", nil, err
	}

	if len(objectiveExpr) != 1 || objectiveExpr[0].Range != "" {
		return "", nil, fmt.Errorf("invalid objective expression")
	}

	constraintExprs := make([]string, 0)
	for _, c := range stmt.Constraints {
		exprs, err := generateObjectiveOrConstraintExpressions(c.ExpressionTokens, c.GroupBy,
			stmt, columns, optFlowVariableStr, optFlowDataStr)
		if err != nil {
			return "", nil, err
		}
		for _, expr := range exprs {
			if expr.Range != "" {
				exprStr := fmt.Sprintf("%s: %s", expr.Range, expr.Expr)
				constraintExprs = append(constraintExprs, exprStr)
			} else {
				constraintExprs = append(constraintExprs, expr.Expr)
			}
		}
	}
	return objectiveExpr[0].Expr, constraintExprs, nil
}

// GenerateOptimizeCode generates optimize codes for execution
func GenerateOptimizeCode(optimStmt *ir.OptimizeStmt, session *pb.Session, tableName string, useOptFlow bool) (string, error) {
	const (
		optimizeTemplateName = "optimize"
	)

	db, err := database.OpenAndConnectDB(session.DbConnStr)
	if err != nil {
		return "", err
	}
	defer db.Close()

	dbName, err := database.GetDatabaseName(session.DbConnStr)
	if err != nil {
		return "", err
	}

	resultTable := optimStmt.ResultTable
	if !strings.Contains(resultTable, ".") {
		resultTable = fmt.Sprintf("%s.%s", dbName, resultTable)
	}

	attrJSON, err := generateOptimizeAttributeJSONString(optimStmt.Attributes)
	if err != nil {
		return "", err
	}

	if !useOptFlow {
		_, err := updateOptimizeStmtByTableColumnNames(optimStmt, db, optimStmt.Select)
		if err != nil {
			return "", err
		}

		filler := pyomoNativeOptimizeFiller{
			DataSource:      session.DbConnStr,
			Select:          optimStmt.Select,
			Variables:       optimStmt.Variables,
			ResultValueName: optimStmt.ResultValueName,
			VariableType:    optimStmt.VariableType,
			Objective:       optimStmt.Objective,
			Direction:       optimStmt.Direction,
			Constraints:     optimStmt.Constraints,
			Solver:          optimStmt.Solver,
			AttributeJSON:   attrJSON,
			ResultTable:     resultTable,
		}
		tpl := template.Must(template.New(optimizeTemplateName).Parse(pyomoNativeOptimizeText))
		var program bytes.Buffer
		if err := tpl.Execute(&program, filler); err != nil {
			return "", err
		}
		return program.String(), nil
	}

	if !strings.Contains(tableName, ".") {
		tableName = fmt.Sprintf("%s.%s", dbName, tableName)
	}

	objective, constraints, err := generateOptFlowObjectiveAndConstraintExpressions(optimStmt, db, tableName)
	if err != nil {
		return "", err
	}

	filler := &optFlowOptimizeFiller{
		UserID:                session.UserId,
		Variables:             optimStmt.Variables,
		ResultValueName:       optimStmt.ResultValueName,
		VariableType:          optimStmt.VariableType,
		Direction:             optimStmt.Direction,
		ObjectiveExpression:   objective,
		ConstraintExpressions: constraints,
		Solver:                optimStmt.Solver,
		AttributeJSON:         attrJSON,
		TrainTable:            tableName,
		ResultTable:           resultTable,
	}

	tpl := template.Must(template.New(optimizeTemplateName).Parse(optFlowOptimizeText))
	var program bytes.Buffer
	if err := tpl.Execute(&program, filler); err != nil {
		return "", err
	}
	return program.String(), nil
}
