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

// GenerateOptimizeCode generates optimize codes for execution
func GenerateOptimizeCode(optimStmt *ir.OptimizeStmt, session *pb.Session, tableName string, useOptFlow bool) (string, error) {
	const (
		optimizeTemplateName = "optimize"
	)

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

	db, err := database.OpenAndConnectDB(session.DbConnStr)
	if err != nil {
		return "", err
	}
	defer db.Close()

	columns, _, err := verifier.GetSQLFieldType(fmt.Sprintf("SELECT * FROM %s", tableName), db)
	if err != nil {
		return "", err
	}

	filler := &optFlowOptimizeFiller{
		UserID:          session.UserId,
		ColumnNames:     columns,
		Variables:       optimStmt.Variables,
		ResultValueName: optimStmt.ResultValueName,
		VariableType:    optimStmt.VariableType,
		Direction:       optimStmt.Direction,
		Objective:       optimStmt.Objective,
		Constraints:     optimStmt.Constraints,
		Solver:          optimStmt.Solver,
		AttributeJSON:   attrJSON,
		TrainTable:      tableName,
		ResultTable:     resultTable,
	}

	tpl := template.Must(template.New(optimizeTemplateName).Parse(optFlowOptimizeText))
	var program bytes.Buffer
	if err := tpl.Execute(&program, filler); err != nil {
		return "", err
	}
	return program.String(), nil
}
