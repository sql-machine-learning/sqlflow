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

import "sqlflow.org/sqlflow/go/ir"

type pyomoNativeOptimizeFiller struct {
	DataSource      string
	Select          string
	Variables       []string
	ResultValueName string
	VariableType    string
	Objective       ir.OptimizeExpr
	Direction       string
	Constraints     []*ir.OptimizeExpr
	Solver          string
	AttributeJSON   string
	ResultTable     string
}

type optFlowOptimizeFiller struct {
	UserID          string
	ColumnNames     []string
	Variables       []string
	ResultValueName string
	VariableType    string
	Objective       ir.OptimizeExpr
	Direction       string
	Constraints     []*ir.OptimizeExpr
	Solver          string
	AttributeJSON   string
	TrainTable      string
	ResultTable     string
}

const optimizeVarObjectiveAndConstraintText = `
variables = [{{range .Variables}}'''{{.}}''',{{end}}]

objective = [{{range .Objective.ExpressionTokens}}'''{{.}}''',{{end}}]

constraints = [{{range .Constraints}}
    {
        "tokens": [{{range .ExpressionTokens}}'''{{.}}''',{{end}}],
        "group_by": '''{{.GroupBy}}''',
    },
{{end}}]
`

const pyomoNativeOptimizeText = optimizeVarObjectiveAndConstraintText + `
from runtime.optimize import run_optimize_locally

run_optimize_locally(datasource='''{{.DataSource}}''', 
                     select='''{{.Select}}''',
                     variables=variables, 
                     variable_type='''{{.VariableType}}''',
                     result_value_name='''{{.ResultValueName}}''',
                     objective=objective,
                     direction='''{{.Direction}}''',
                     constraints=constraints,
                     solver='''{{.Solver}}''',
                     result_table='''{{.ResultTable}}''')
`

const optFlowOptimizeText = optimizeVarObjectiveAndConstraintText + `
from runtime.optimize import run_optimize_on_optflow

columns = [{{range .ColumnNames}}'''{{.}}''',{{end}}]

run_optimize_on_optflow(train_table='''{{.TrainTable}}''',
                        columns=columns,
                        variables=variables,
                        variable_type='''{{.VariableType}}''',
                        result_value_name='''{{.ResultValueName}}''',
                        objective=objective,
                        direction='''{{.Direction}}''',
                        constraints=constraints,
                        solver='''{{.Solver}}''',
                        result_table='''{{.ResultTable}}''',
                        user_number='''{{.UserID}}''')
`
