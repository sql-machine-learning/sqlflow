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
import os
import json
import sys
from optflow.core.api.config import (InputConf, OdpsItemConf, OdpsConf, OutputConf, RunnerConf, SolverConf,
                                     SolverExperiment, OptflowLocalEngine, OptflowKubemakerEngine, OptionConf)
from optflow.core.submit import submit_experiment

from alps.framework.engine import ResourceConf

def submit():
    attributes = json.loads('''{{.AttributeJSON}}''')

    solver_options = attributes.get("solver", {})
    sys.stderr.write('solver options: {}\n'.format(solver_options))
    solver_conf = SolverConf(name="{{.Solver}}", options=OptionConf(solver_options))

    pai_project = "{{.TrainTable}}".split('.')[0]
    odps_conf = OdpsConf(project=pai_project,
                         accessid=os.environ.get("SQLFLOW_TEST_DB_MAXCOMPUTE_AK"),
                         accesskey=os.environ.get("SQLFLOW_TEST_DB_MAXCOMPUTE_SK"),
                         partitions=None)
    
    runner = RunnerConf(cls="{{.RunnerModule}}.CustomOptFlowRunner")

    data_options = attributes.get("data", {})
    sys.stderr.write('data options: {}\n'.format(data_options))
    enable_slice = data_options.get("enable_slice", False)
    batch_size = data_options.get("batch_size", None)
    if batch_size <= 0:
         batch_size = None

    output_table = OdpsItemConf(path="odps://{{.ResultTable}}", odps=odps_conf)
    output = OutputConf(df1=output_table)

    df1 = OdpsItemConf(path="odps://{{.TrainTable}}",
                       odps=odps_conf,
                       enable_slice=enable_slice,
                       batch_size=batch_size)
    
    input_conf = InputConf(df1=df1)

    optflow_version = os.environ.get("SQLFLOW_OPTFLOW_VERSION")
    if not optflow_version:
        raise ValueError("Environment variable SQLFLOW_OPTFLOW_VERSION must be set")
    	
    cluster = os.environ.get("SQLFLOW_OPTFLOW_KUBEMAKER_CLUSTER")
    if not cluster:
        raise ValueError("Environment variable SQLFLOW_OPTFLOW_KUBEMAKER_CLUSTER must be set")
 
    worker_options = attributes.get("worker", {})
    sys.stderr.write('worker options: {}\n'.format(worker_options))
    # TODO(sneaxiy): support local engine
    engine = OptflowKubemakerEngine(worker=ResourceConf(**worker_options), cluster=cluster)

    user_id = "{{.UserID}}"
    if not user_id:
        user_id = "jinle.zjl"

    experiment = SolverExperiment(user=user_id,
                                  engine=engine,
                                  runner=runner,
                                  solver=solver_conf,
                                  input_conf=input_conf,
                                  output_conf=output)

    submit_experiment(experiment, optflow_version=optflow_version)

if __name__ == '__main__':
    submit()
`
