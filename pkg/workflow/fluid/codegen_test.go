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

package fluid

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

const (
	expectedFluid = `
import fluid

step_envs = dict()

step_envs["SQLFLOW_DATASOURCE"] = '''mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0'''

step_envs["SQLFLOW_OSS_AK"] = '''oss_key'''

step_envs["SQLFLOW_TEST"] = '''workflow'''

step_envs["SQLFLOW_TEST_DATASOURCE"] = '''mysql://root:root@tcp(172.17.0.9:3306)/?maxAllowedPacket=0'''

step_envs["SQLFLOW_TEST_DB"] = '''mysql'''

step_envs["SQLFLOW_submitter"] = ''''''


@fluid.task
def sqlflow_workflow():

    fluid.step(image="sqlflow/sqlflow:step", cmd=["step"], args=["-e", '''SELECT * FROM iris.train limit 10;'''], env=step_envs)

    fluid.step(image="sqlflow/sqlflow:step", cmd=["step"], args=["-e", '''SELECT * FROM iris_train
TO TRAIN DNNClassifier WITH
	train.batch_size=4,
	train.epoch=3,
	model.hidden_units=[10,20],
	model.n_classes=3
LABEL class
INTO my_dnn_model;
	'''], env=step_envs)


sqlflow_workflow()
`
	expectedYAML = `---
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: sqlflow-workflow
spec:
  inputs:
    params: []
    resources: []
  outputs:
    resources: []
  steps:
  - args:
    - -e
    - SELECT * FROM iris.train limit 10;
    command:
    - step
    env:
    - name: SQLFLOW_DATASOURCE
      value: mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0
    - name: SQLFLOW_OSS_AK
      value: oss_key
    - name: SQLFLOW_TEST
      value: workflow
    - name: SQLFLOW_TEST_DATASOURCE
      value: mysql://root:root@tcp(172.17.0.9:3306)/?maxAllowedPacket=0
    - name: SQLFLOW_TEST_DB
      value: mysql
    - name: SQLFLOW_submitter
      value: ''
    image: sqlflow/sqlflow:step
    name: <stdin>-22
  - args:
    - -e
    - "SELECT * FROM iris_train\nTO TRAIN DNNClassifier WITH\n\ttrain.batch_size=4,\n\
      \ttrain.epoch=3,\n\tmodel.hidden_units=[10,20],\n\tmodel.n_classes=3\nLABEL\
      \ class\nINTO my_dnn_model;\n\t"
    command:
    - step
    env:
    - name: SQLFLOW_DATASOURCE
      value: mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0
    - name: SQLFLOW_OSS_AK
      value: oss_key
    - name: SQLFLOW_TEST
      value: workflow
    - name: SQLFLOW_TEST_DATASOURCE
      value: mysql://root:root@tcp(172.17.0.9:3306)/?maxAllowedPacket=0
    - name: SQLFLOW_TEST_DB
      value: mysql
    - name: SQLFLOW_submitter
      value: ''
    image: sqlflow/sqlflow:step
    name: <stdin>-32
---
apiVersion: tekton.dev/v1alpha1
kind: TaskRun
metadata:
  name: sqlflow-workflow-run
spec:
  inputs:
    params: []
    resources: []
  outputs:
    resources: []
  taskRef:
    name: sqlflow-workflow
`
)

func TestFluidCodegen(t *testing.T) {
	a := assert.New(t)
	// Test step environment variables, the prefix `SQLFLOW_WORKFLOW_` would not be in step container
	os.Setenv("SQLFLOW_OSS_AK", "oss_key")
	os.Setenv("SQLFLOW_WORKFLOW_STEP_IMAGE", "sqlflow/sqlflow:step")

	sqlIR := MockSQLProgramIR()
	cg := &Codegen{}
	code, err := cg.GenCode(sqlIR, &pb.Session{})
	a.NoError(err)
	a.Equal(expectedFluid, code)

	yaml, e := cg.GenYAML(code)
	a.NoError(e)
	a.Equal(expectedYAML, yaml)
}
