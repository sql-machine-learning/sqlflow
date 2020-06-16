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
	DataSource      string
	Select          string
	Variables       []string
	ResultValueName string
	VariableType    string
	Objective       string
	Direction       string
	Constraints     []*ir.OptimizeExpr
	Solver          string
	ResultTable     string
	IsPAI           bool
	PAITrainTable   string
}

const paiOptFlowRunnerText = `
from sqlflow_submitter.optimize import BaseOptFlowRunner

__all__ = ['CustomOptFlowRunner']

class CustomOptFlowRunner(BaseOptFlowRunner):
    def init_parameters(self):
        self.variables = [{{range .Variables}}
            "{{.}}",
        {{end}}]

        self.result_value_name = "{{.ResultValueName}}"

        self.variable_type = "{{.VariableType}}"

        self.direction = "{{.Direction}}"

        self.objective = "{{.Objective}}"

        self.constraints = [{{range .Constraints}}
            {"expression": "{{.Expression}}", "group_by": "{{.GroupBy}}"},
        {{end}}]
`

const paiOptFlowSubmitText = `
import os
from alps.framework.engine import ResourceConf
from optflow.core.api.config import (InputConf, OdpsItemConf, OdpsConf, OutputConf, RunnerConf, SolverConf,
                                     SolverExperiment, OptflowLocalEngine, OptflowKubemakerEngine, OptionConf)
from optflow.core.submit import submit_experiment

def submit():
    options = {}  # solver options
    solver_conf = SolverConf(name="{{.Solver}}", options=OptionConf(options))

    pai_project = "{{.PAITrainTable}}".split('.')[0]
    odps_conf = OdpsConf(project=pai_project,
                         accessid=os.environ.get("SQLFLOW_TEST_DB_MAXCOMPUTE_AK"),
                         accesskey=os.environ.get("SQLFLOW_TEST_DB_MAXCOMPUTE_SK"),
                         partitions=None)
    
    runner = RunnerConf(cls="custom_optimize_runner.CustomOptFlowRunner")
    
    output_table = OdpsItemConf(path="odps://{{.ResultTable}}", odps=odps_conf)
    output = OutputConf(df1=output_table)

    df1 = OdpsItemConf(path="odps://{{.PAITrainTable}}",
                       odps=odps_conf,
                       enable_slice=False)
    
    input_conf = InputConf(df1=df1)
    
    # TODO(sneaxiy): support local engine
    # engine = OptflowLocalEngine()
	
    cluster = os.environ.get("SQLFLOW_OPTFLOW_KUBEMAKER_CLUSTER")
    optflow_version = os.environ.get("SQLFLOW_OPTFLOW_VERSION")

    if not cluster or not optflow_version:
        raise ValueError("Environment variable SQLFLOW_OPTFLOW_KUBEMAKER_CLUSTER and SQLFLOW_OPTFLOW_VERSION must be set")
    
    # TODO(sneaxiy): move ResourceConf setting to WITH statements
    engine = OptflowKubemakerEngine(worker=ResourceConf(core=8, memory=20000, num=1), cluster=cluster)
    
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
