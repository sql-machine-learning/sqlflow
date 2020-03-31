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

package deps

import (
	"fmt"

	"sqlflow.org/sqlflow/pkg/parser"
)

// SQLProgram is the constructed graph of the SQL program.
type SQLProgram struct {
	Statements []*Statement
}

// Statement represents a graph node of one statement.
type Statement struct {
	Statement string
	// Statement's input/output must be a table.
	Inputs  []*Table
	Outputs []*Table
}

// TableType can be table or model
type TableType string

const (
	// TypeTable is a table node
	TypeTable = "table"
	// TypeModel is a TypeModel
	TypeModel = "model"
)

// Table represents a graph node of one database table.
type Table struct {
	// Type can be "table" or "model".
	Type TableType
	Name string
	// Table's input/output must be a statement.
	Inputs  []*Statement
	Outputs []*Statement
}

// FullName of the table node.
func (t *Table) FullName() string {
	return string(t.Type) + "." + t.Name
}

// Analyze will construct a dependency graph for the SQL program and
// returns the first statement (root node).
func Analyze(parsedProgram []*parser.SQLFlowStmt) (*Statement, error) {
	if len(parsedProgram) == 0 {
		return nil, fmt.Errorf("no parsed statements to analyze")
	}
	if len(parsedProgram) == 1 {
		return &Statement{
			Statement: parsedProgram[0].Original,
			Inputs:    nil,
			Outputs:   nil,
		}, nil
	}

	var root *Statement
	tableNodeMap := make(map[string]*Table)

	for idx, stmt := range parsedProgram {
		inputs := []*Table{}
		outputs := []*Table{}
		for _, t := range stmt.Inputs {
			fullName := "table." + t
			var tableNode *Table
			tableNode, ok := tableNodeMap[fullName]
			if !ok {
				tableNode = &Table{
					Type:    TypeTable,
					Name:    t,
					Inputs:  []*Statement{},
					Outputs: []*Statement{},
				}
				tableNodeMap[fullName] = tableNode
			}
			inputs = append(inputs, tableNode)
		}
		for _, t := range stmt.Outputs {
			fullName := "table." + t
			var tableNode *Table
			tableNode, ok := tableNodeMap[fullName]
			if !ok {
				tableNode = &Table{
					Type:    TypeTable,
					Name:    t,
					Inputs:  []*Statement{},
					Outputs: []*Statement{},
				}
				tableNodeMap[fullName] = tableNode
			}
			outputs = append(outputs, tableNode)
		}
		curr := &Statement{
			Statement: stmt.Original,
			Inputs:    inputs,
			Outputs:   outputs,
		}
		if idx == 0 {
			root = curr
		}
	}
	return root, nil
}
