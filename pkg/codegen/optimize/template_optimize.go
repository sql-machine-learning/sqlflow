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

import "sqlflow.org/sqlflow/pkg/ir"

type optimizeFiller struct {
	UserID          string
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
	RunnerModule    string
}

const optFlowRunnerText = `
from sqlflow_submitter.optimize import BaseOptFlowRunner

__all__ = ['CustomOptFlowRunner']

class CustomOptFlowRunner(BaseOptFlowRunner):
    def init_parameters(self):
        self.variables = [{{range .Variables}}"{{.}}",{{end}}]

        self.result_value_name = "{{.ResultValueName}}"

        self.variable_type = "{{.VariableType}}"

        self.direction = "{{.Direction}}"

        self.objective = [{{range .Objective.ExpressionTokens}}"{{.}}",{{end}}]

        self.constraints = [{{range .Constraints}}
            {
                "expression": [{{range .ExpressionTokens}}"{{.}}",{{end}}],
                "group_by": "{{.GroupBy}}",
            },
        {{end}}]
`

const optFlowSubmitText = `
import json
from sqlflow_submitter.optimize import submit

runner = "{{.RunnerModule}}.CustomOptFlowRunner"
solver = "{{.Solver}}"
attributes = json.loads('''{{.AttributeJSON}}''')
train_table = "{{.TrainTable}}"
result_table = "{{.ResultTable}}"

user_id = "{{.UserID}}"
if not user_id:
    user_id = "jinle.zjl"

submit(runner=runner, 
       solver=solver, 
       attributes=attributes, 
       train_table=train_table,
       result_table=result_table,
       user_id=user_id)
`
