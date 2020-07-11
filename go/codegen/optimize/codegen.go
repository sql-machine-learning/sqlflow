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
	"reflect"
	"sqlflow.org/sqlflow/go/attribute"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
	"sqlflow.org/sqlflow/go/verifier"
	"strconv"
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

// convertRowDataToList converts the row data read from db to be one of []int64,
// []float64 and []string
func convertRowDataToList(rowData []interface{}) (interface{}, error) {
	var columnType int
	switch firstCellType := rowData[0].(type) {
	case *string:
		columnType = ir.String
	case *int32:
		columnType = ir.Int
	case *int64:
		columnType = ir.Int
	case *float32:
		columnType = ir.Float
	case *float64:
		columnType = ir.Float
	default:
		return nil, fmt.Errorf("not supported rowData type %T", firstCellType)
	}

	if columnType == ir.Int {
		result := make([]int64, len(rowData))
		for i, cell := range rowData {
			switch v := cell.(type) {
			case *int32:
				result[i] = int64(*v)
			case *int64:
				result[i] = int64(*v)
			default:
				return nil, fmt.Errorf("not supported rowData type %T", v)
			}
		}
		return result, nil
	}

	if columnType == ir.Float {
		floatResult := make([]float64, len(rowData))
		isFloat := false
		for i, cell := range rowData {
			switch v := cell.(type) {
			case *float32:
				floatResult[i] = float64(*v)
			case *float64:
				floatResult[i] = float64(*v)
			default:
				return nil, fmt.Errorf("not supported rowData type %T", v)
			}

			if floatResult[i] != float64(int64(floatResult[i])) {
				isFloat = true
			}
		}

		if isFloat {
			return floatResult, nil
		}
		intResult := make([]int64, len(floatResult))
		for i, v := range floatResult {
			intResult[i] = int64(v)
		}
		return intResult, nil
	}

	strValue := make([]string, len(rowData))
	isNumber := true
	isFloat := false
	for i, cell := range rowData {
		strCell, ok := cell.(*string)
		if !ok {
			return nil, fmt.Errorf("not supported rowData type %T", strCell)
		}
		strValue[i] = *strCell
		if !isNumber {
			continue
		}

		floatValue, err := strconv.ParseFloat(strValue[i], 64)
		if err != nil {
			isNumber = false
			continue
		}

		if !isFloat {
			if floatValue != float64(int64(floatValue)) {
				isFloat = true
			}
		}
	}

	if !isNumber {
		return strValue, nil
	}

	if isFloat {
		floatResult := make([]float64, len(strValue))
		for i, v := range strValue {
			floatResult[i], _ = strconv.ParseFloat(v, 64)
		}
		return floatResult, nil
	}

	intResult := make([]int64, len(strValue))
	for i, v := range strValue {
		intResult[i], _ = strconv.ParseInt(v, 10, 64)
	}
	return intResult, nil
}

// getGroupByIndexRanges generates the data range of GROUP BY
// Supposing that we have a table:
//   +-----+-----+
//   |  A  |  B  |
//   +-----+-----+
//   | "x" | "z" |
//   | "x" | "w" |
//   | "y" | "z" |
//   | "y" | "w" |
//   +-----+-----+
// If groupByColumns = ["A"], we would get {"A": [[0, 1], [2, 3]]}
// If groupByColumns = ["B"], we would get {"B": [[0, 2], [1, 3]]}
func getGroupByIndexRanges(tableData map[string]interface{}, groupByColumns []string) (map[string][][]int, error) {
	isEqual := func(list interface{}, i int, j int) bool {
		switch v := list.(type) {
		case []int64:
			return v[i] == v[j]
		case []float64:
			return v[i] == v[j]
		case []string:
			return v[i] == v[j]
		default:
			// not important, because v must be type of []int64, []float64 or []string
			return false
		}
	}

	result := make(map[string][][]int)
	for _, groupBy := range groupByColumns {
		groupByValues, ok := tableData[groupBy]
		if !ok {
			return nil, fmt.Errorf("cannot find GROUP BY column %s", groupBy)
		}

		uniqueValueIndices := make([][]int, 0)
		length := reflect.ValueOf(groupByValues).Len()
		for i := 0; i < length; i++ {
			existIndex := -1
			for j, indices := range uniqueValueIndices {
				if isEqual(groupByValues, i, indices[0]) {
					existIndex = j
					break
				}
			}

			if existIndex >= 0 {
				uniqueValueIndices[existIndex] = append(uniqueValueIndices[existIndex], i)
			} else {
				uniqueValueIndices = append(uniqueValueIndices, []int{i})
			}
		}

		result[groupBy] = uniqueValueIndices
	}
	return result, nil
}

// getTableDataAndGroupByIndexRanges reads all data from the table
// and returns (tableData, groupByIndexRanges, error)
func getTableDataAndGroupByIndexRanges(stmt *ir.OptimizeStmt, columns []string, db *database.DB, tableName string) (
	map[string]interface{}, map[string][][]int, error) {
	groupByColumns := make([]string, 0)
	for _, c := range stmt.Constraints {
		if c.GroupBy != "" {
			hasAggregationFunction := false
			for _, token := range c.ExpressionTokens {
				if tryConvertToAggregationFunction(token) != "" {
					hasAggregationFunction = true
					break
				}
			}

			if !hasAggregationFunction {
				return nil, nil, fmt.Errorf("GROUP BY %s must be used with aggregation function together", c.GroupBy)
			}

			exists := false
			for _, name := range groupByColumns {
				if c.GroupBy == name {
					exists = true
				}
			}

			// only find unique GROUP BY column names
			if !exists {
				groupByColumns = append(groupByColumns, c.GroupBy)
			}
		}
	}

	// If there is no GROUP BY, table data is not necessary for generating the optimize model code.
	if len(groupByColumns) == 0 {
		return nil, nil, nil
	}

	selectStmt := fmt.Sprintf("SELECT %s FROM %s", strings.Join(columns, ","), tableName)
	rows, err := verifier.FetchNSamples(db, selectStmt, -1)
	if err != nil {
		return nil, nil, err
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, nil, err
	}

	allRowData := make([][]interface{}, len(columns))
	for rows.Next() {
		rowData, err := ir.ScanRowValue(rows, columnTypes)
		if err != nil {
			return nil, nil, err
		}
		for i, cellValue := range rowData {
			allRowData[i] = append(allRowData[i], cellValue)
		}
	}

	tableData := make(map[string]interface{})
	for i, rowData := range allRowData {
		valueList, err := convertRowDataToList(rowData)
		if err != nil {
			return nil, nil, err
		}
		tableData[columns[i]] = valueList
	}

	ranges, err := getGroupByIndexRanges(tableData, groupByColumns)
	if err != nil {
		return nil, nil, err
	}
	return tableData, ranges, err
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
func generateTokenInNonAggregationExpression(token string, stmt *ir.OptimizeStmt, columns []string,
	tableData map[string]interface{}, indices []int, dataStr string) (string, error) {
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

		if indices == nil || len(indices) == 0 {
			return "", fmt.Errorf(
				"invalid expression: column %s should not occur in the non-aggregation part of constraint clause without GROUP BY", token)
		}

		if values, ok := tableData[token]; ok {
			// return the value of the first row when using GROUP BY
			return fmt.Sprintf("%v", reflect.ValueOf(values).Index(indices[0]).Interface()), nil
		}
		return "", fmt.Errorf("cannot find column %s", token)
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
func generateNonAggregatedConstraintExpression(tokens []string, stmt *ir.OptimizeStmt, columns []string, variableStr string, dataStr string) (string, error) {
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
	return strings.Join(resultTokens, ""), nil
}

// generateObjectiveOrAggregatedConstraintExpression generates the expression from tokens with aggregation functions.
// Input SQL statement: TO MAXIMIZE sum((price - cost) * amount)
// Output expression: sum([(@input["price"] - @input["cost"]) * @X[i] for i in @X]), if variableStr = "@X" and dataStr = "@input"
// If there is GROUP BY in the CONSTRAINT clause, indices must be provided.
func generateObjectiveOrAggregatedConstraintExpression(tokens []string, stmt *ir.OptimizeStmt, columns []string,
	tableData map[string]interface{}, indices []int, variableStr string, dataStr string) (string, error) {
	idx := 0
	resultTokens := make([]string, 0)

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
			return "", err
		}

		leftBracketIdx := nextIdx
		rightBracketIdx := nextIdx
		if len(leftBracketIndices) != 0 {
			leftBracketIdx = leftBracketIndices[0]
			rightBracketIdx = rightBracketIndices[0]
		}

		for ; idx < leftBracketIdx; idx++ {
			token, err := generateTokenInNonAggregationExpression(tokens[idx], stmt, columns, tableData, indices, dataStr)
			if err != nil {
				return "", err
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
				return "", err
			}

			if tokens[idx] == ")" {
				if contains(rightBracketIndices, idx) { // right bracket of the SUM(...)
					resultTokens = append(resultTokens, " ")
					forRangeStr := ""
					if indices == nil || len(indices) == 0 {
						forRangeStr = fmt.Sprintf(`for i_%d in %s`, depth, variableStr)
					} else {
						jsonIndices, err := json.Marshal(indices)
						if err != nil {
							return "", err
						}
						forRangeStr = fmt.Sprintf(`for i_%d in %s`, depth, jsonIndices)
					}
					resultTokens = append(resultTokens, forRangeStr, "]")
				}
				resultTokens = append(resultTokens, tokens[idx])
				continue
			}

			token, err := generateTokenInAggregationExpression(tokens[idx], stmt, columns, variableStr, dataStr, depth)
			if err != nil {
				return "", err
			}
			resultTokens = append(resultTokens, token)
		}

		for idx = rightBracketIdx + 1; idx < nextIdx; idx++ {
			token, err := generateTokenInNonAggregationExpression(tokens[idx], stmt, columns, tableData, indices, dataStr)
			if err != nil {
				return "", err
			}
			resultTokens = append(resultTokens, token)
		}
	}

	return strings.Join(resultTokens, ""), nil
}

type rangedExpression struct {
	Expr     string
	HasRange bool
}

// generateObjectiveOrConstraintExpressions generates expression of objective or constraint
func generateObjectiveOrConstraintExpressions(tokens []string, groupBy string, stmt *ir.OptimizeStmt, columns []string,
	tableData map[string]interface{},
	groupByRanges map[string][][]int, variableStr string, dataStr string) ([]*rangedExpression, error) {
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

		rang, ok := groupByRanges[groupBy]
		if !ok {
			return nil, fmt.Errorf("cannot find GROUP BY column %s", groupBy)
		}

		for _, r := range rang {
			expr, err := generateObjectiveOrAggregatedConstraintExpression(tokens, stmt, columns, tableData, r, variableStr, dataStr)
			if err != nil {
				return nil, err
			}
			expressions = append(expressions, &rangedExpression{Expr: expr})
		}
		return expressions, nil
	}

	if hasAggregationFunc {
		expr, err := generateObjectiveOrAggregatedConstraintExpression(tokens, stmt, columns, tableData, nil, variableStr, dataStr)
		if err != nil {
			return nil, err
		}
		expressions = append(expressions, &rangedExpression{Expr: expr})
	} else {
		expr, err := generateNonAggregatedConstraintExpression(tokens, stmt, columns, variableStr, dataStr)
		if err != nil {
			return nil, err
		}
		expressions = append(expressions, &rangedExpression{Expr: expr, HasRange: true})
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

	tableData, groupByRanges, err := getTableDataAndGroupByIndexRanges(stmt, columns, db, tableName)
	if err != nil {
		return "", nil, err
	}

	objectiveExpr, err := generateObjectiveOrConstraintExpressions(stmt.Objective.ExpressionTokens, "",
		stmt, columns, tableData, groupByRanges, optFlowVariableStr, optFlowDataStr)
	if err != nil {
		return "", nil, err
	}

	if len(objectiveExpr) != 1 || objectiveExpr[0].HasRange {
		return "", nil, fmt.Errorf("invalid objective expression")
	}

	constraintExprs := make([]string, 0)
	for _, c := range stmt.Constraints {
		exprs, err := generateObjectiveOrConstraintExpressions(c.ExpressionTokens, c.GroupBy,
			stmt, columns, tableData, groupByRanges, optFlowVariableStr, optFlowDataStr)
		if err != nil {
			return "", nil, err
		}
		for _, expr := range exprs {
			if expr.HasRange {
				exprStr := fmt.Sprintf("for i in %s: %s", optFlowVariableStr, expr.Expr)
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
