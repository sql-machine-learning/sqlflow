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
	"os"
	"strings"

	"sqlflow.org/sqlflow/pkg/parser"
)

// Statement represents a graph node of one statement.
type Statement struct {
	Statement string
	// index of the statement in the SQL program
	Order int
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
	Type        TableType
	Name        string
	HazardIndex int
	// Table's input/output must be a statement.
	// Inputs  []*Statement
	// Outputs []*Statement
}

// FullName of the table node.
func (t *Table) FullName() string {
	return string(t.Type) + "." + t.Name
}

// Analyze will construct a dependency graph for the SQL program and
// returns the first statement (root node).
func Analyze(parsedProgram []*parser.SQLFlowStmt) ([]*Statement, error) {
	if len(parsedProgram) == 0 {
		return nil, fmt.Errorf("no parsed statements to analyze")
	}
	var result []*Statement

	if len(parsedProgram) == 1 {
		result = append(result, &Statement{
			Statement: parsedProgram[0].Original,
			Order:     0,
			Inputs:    nil,
			Outputs:   nil,
		})
		return result, nil
	}

	// tableNodeMap records table fullname -> fullname_0, fullname_1 for resolving hazard.
	tableNodeMap := make(map[string][]*Table)

	for idx, stmt := range parsedProgram {
		inputs := []*Table{}
		outputs := []*Table{}
		for _, t := range stmt.Inputs {
			fullName := "table." + t
			tableNodelist, ok := tableNodeMap[fullName]
			if !ok {
				tableNode := &Table{
					Type:        TypeTable,
					Name:        t,
					HazardIndex: 0,
				}
				tableNodeMap[fullName] = []*Table{tableNode}
				inputs = append(inputs, tableNode)
			} else {
				inputs = append(inputs, tableNodelist[len(tableNodelist)-1])
			}
		}
		for _, t := range stmt.Outputs {
			fullName := "table." + t
			tableNodeList, ok := tableNodeMap[fullName]
			if !ok {
				tableNode := &Table{
					Type:        TypeTable,
					Name:        t,
					HazardIndex: 0,
				}
				tableNodeMap[fullName] = []*Table{tableNode}
				outputs = append(outputs, tableNode)
				continue
			}
			// find last statement that read/write this table
			for j := len(result) - 1; j >= 0; j-- {
				prev := result[j]
				tableNodeList = tableNodeMap[fullName]
				if contains(prev.Outputs, fullName) {
					fmt.Printf("prev append WAW: %s, table: %s\n", prev.Statement, fullName)
					// WAW solution
					prevOutput, ok := getFirstInList(prev.Outputs, fullName)
					if !ok {
						return nil, fmt.Errorf("error adding WAW dependency, tablename: %s, prev stmt: %s", fullName, prev.Statement)
					}
					inputs = append(inputs, prevOutput)
				} else if contains(prev.Inputs, fullName) {
					fmt.Printf("prev append WAR: %s, table: %s\n", prev.Statement, fullName)
					// WAR solution
					readAsOutputTable := &Table{
						Type:        TypeTable,
						Name:        t,
						HazardIndex: tableNodeList[len(tableNodeList)-1].HazardIndex + 1,
					}
					prev.Outputs = append(prev.Outputs, readAsOutputTable)
					if !contains(inputs, fullName) {
						inputs = append(inputs, readAsOutputTable)
					}
					tableNodeMap[fullName] = append(tableNodeMap[fullName], readAsOutputTable)
				}
			}
			tableNodeList = tableNodeMap[fullName]
			warTable := &Table{
				Type:        TypeTable,
				Name:        t,
				HazardIndex: tableNodeList[len(tableNodeList)-1].HazardIndex + 1,
			}
			outputs = append(outputs, warTable)
			tableNodeMap[fullName] = append(tableNodeMap[fullName], warTable)
		}
		curr := &Statement{
			Statement: stmt.Original,
			Order:     idx,
			Inputs:    inputs,
			Outputs:   outputs,
		}
		result = append(result, curr)
	}
	if err := drawGraphviz(result); err != nil {
		return result, err
	}
	return result, nil
}

func getFirstInList(tableList []*Table, tableNameToFind string) (*Table, bool) {
	for _, t := range tableList {
		if t.FullName() == tableNameToFind {
			return t, true
		}
	}
	return nil, false
}

func contains(tableList []*Table, item string) bool {
	res := false
	for _, t := range tableList {
		if t.FullName() == item {
			res = true
			break
		}
	}
	return res
}

func drawGraphviz(stmts []*Statement) error {
	dotFilePath := os.Getenv("SQLFLOW_GRAPHVIZ_OUTPUT")
	if dotFilePath == "" {
		// skip if no SQLFLOW_GRAPHVIZ_OUTPUT set
		return nil
	}
	fn, err := os.Create(dotFilePath)
	if err != nil {
		return err
	}
	defer fn.Close()
	if _, err = fn.WriteString("digraph D {\n"); err != nil {
		return err
	}

	// store unique table names
	tableNames := make(map[string]int)
	// write stmt nodes
	for _, stmt := range stmts {
		stmtLine := fmt.Sprintf("Stmt%d [shape=box label=\"%s\"]\n", stmt.Order, strings.Trim(stmt.Statement, "\n"))
		if _, err = fn.WriteString(stmtLine); err != nil {
			return err
		}
		if len(stmt.Inputs) > 0 {
			for _, t := range stmt.Inputs {
				fullName := fmt.Sprintf("%s_%d", t.FullName(), t.HazardIndex)
				if _, ok := tableNames[fullName]; !ok {
					tableNames[fullName] = 1
				}
			}
		}
		if len(stmt.Outputs) > 0 {
			for _, t := range stmt.Outputs {
				fullName := fmt.Sprintf("%s_%d", t.FullName(), t.HazardIndex)
				if _, ok := tableNames[fullName]; !ok {
					tableNames[fullName] = 1
				}
			}
		}
	}
	// write table nodes
	for tn := range tableNames {
		tableLine := fmt.Sprintf("%s [shape=circle]\n", strings.Replace(tn, ".", "_", -1))
		if _, err = fn.WriteString(tableLine); err != nil {
			return err
		}
	}

	// write edges
	for _, stmt := range stmts {
		// write inputs
		if len(stmt.Inputs) > 0 {
			for _, i := range stmt.Inputs {
				inputLine := fmt.Sprintf("%s_%d -> Stmt%d\n", strings.Replace(i.FullName(), ".", "_", -1), i.HazardIndex, stmt.Order)
				if _, err = fn.WriteString(inputLine); err != nil {
					return err
				}
			}
		}

		// write outputs
		if len(stmt.Outputs) > 0 {
			for _, o := range stmt.Outputs {
				inputLine := fmt.Sprintf("Stmt%d -> %s_%d\n", stmt.Order, strings.Replace(o.FullName(), ".", "_", -1), o.HazardIndex)
				if _, err = fn.WriteString(inputLine); err != nil {
					return err
				}
			}
		}
	}
	if _, err = fn.WriteString("}\n"); err != nil {
		return err
	}
	return nil
}
