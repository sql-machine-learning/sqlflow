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

package experimental

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/template"

	pb "sqlflow.org/sqlflow/go/proto"
	"sqlflow.org/sqlflow/go/workflow/couler"
)

var workflowTTL = 24 * 3600

type stepContext struct {
	Code      string
	StepIndex int
	Image     string
}

type coulerFiller struct {
	StepList         []*stepContext
	DataSource       string
	StepEnvs         map[string]string
	WorkflowTTL      int
	SecretName       string
	SecretData       string
	Resources        string
	StepLogFile      string
	StepExitTimeWait int64
}

// GenerateCodeCouler generate a Couler program to submit a workflow to run the sql program.
// 1. generate IR of each statement.
// 2. generate runtime code of each statement
// 3. generate couler program to form a workflow
func GenerateCodeCouler(sqlProgram string, session *pb.Session) (string, error) {
	defaultDockerImage := os.Getenv("SQLFLOW_WORKFLOW_STEP_IMAGE")
	if defaultDockerImage == "" {
		defaultDockerImage = "sqlflow/sqlflow:step"
	}
	stmts, err := parseToIR(sqlProgram, session)
	if err != nil {
		return "", err
	}
	var stepList []*stepContext
	for idx, stmt := range stmts {
		stepCode, image, err := GenerateStepCodeAndImage(stmt, idx, session, stmts)
		if err != nil {
			return "", err
		}
		if image == "" {
			image = defaultDockerImage
		}
		step := &stepContext{
			Code:      stepCode,
			Image:     image,
			StepIndex: idx,
		}
		stepList = append(stepList, step)
	}
	return CodeGenCouler(stepList, session)
}

// CodeGenCouler generate couler code to generate a workflow
func CodeGenCouler(stepList []*stepContext, session *pb.Session) (string, error) {
	var workflowResourcesEnv = "SQLFLOW_WORKFLOW_RESOURCES"
	envs, err := couler.GetStepEnvs(session)
	if err != nil {
		return "", err
	}
	secretName, secretData, err := couler.GetSecret()
	if err != nil {
		return "", err
	}
	if err := couler.VerifyResources(os.Getenv(workflowResourcesEnv)); err != nil {
		return "", err
	}
	if os.Getenv("SQLFLOW_WORKFLOW_TTL") != "" {
		workflowTTL, err = strconv.Atoi(os.Getenv("SQLFLOW_WORKFLOW_TTL"))
		if err != nil {
			return "", fmt.Errorf("SQLFLOW_WORKFLOW_TTL: %s should be int", os.Getenv("SQLFLOW_WORKFLOW_TTL"))
		}
	}

	exitTimeWait := int64(0)
	exitTimeWaitEnv := os.Getenv("SQLFLOW_WORKFLOW_EXIT_TIME_WAIT")
	if exitTimeWaitEnv != "" {
		exitTimeWait, err = strconv.ParseInt(exitTimeWaitEnv, 10, 64)
		if err != nil {
			return "", fmt.Errorf("SQLFLOW_WORKFLOW_EXIT_TIME_WAIT: %s should be int", exitTimeWaitEnv)
		}
	}

	filler := &coulerFiller{
		StepList:         stepList,
		DataSource:       session.DbConnStr,
		StepEnvs:         envs,
		WorkflowTTL:      workflowTTL,
		SecretName:       secretName,
		SecretData:       secretData,
		Resources:        os.Getenv(workflowResourcesEnv),
		StepLogFile:      os.Getenv("SQLFLOW_WORKFLOW_STEP_LOG_FILE"),
		StepExitTimeWait: exitTimeWait,
	}
	var program bytes.Buffer
	if err := coulerTemplate.Execute(&program, filler); err != nil {
		return "", err
	}
	return program.String(), nil
}

// GetPyFuncBody gets the Python function body
func GetPyFuncBody(program string, funcName string) (string, error) {
	const coulerGetPyFuncCodeImpl = `
%s

import couler.pyfunc as pyfunc
print(pyfunc.body(%s))
`

	tmpFile, err := ioutil.TempFile("/tmp", "sqlflow-couler-tmp")
	if err != nil {
		return "", err
	}

	defer tmpFile.Close()
	defer os.RemoveAll(tmpFile.Name())

	coulerCode := fmt.Sprintf(coulerGetPyFuncCodeImpl, program, funcName)
	_, err = tmpFile.Write([]byte(coulerCode))
	if err != nil {
		return "", err
	}
	cmd := exec.Command("python", tmpFile.Name())
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%v: %s\nCode is:\n%s", err, stderr, coulerCode)
	}
	return strings.TrimSpace(stdout.String()), nil
}

const coulerCodeTmpl = `
import couler.argo as couler
import couler.pyfunc as pyfunc
from os import path
import json
import re

datasource = "{{ .DataSource }}"

step_envs = dict()
{{range $k, $v := .StepEnvs}}step_envs["{{$k}}"] = '''{{$v}}'''
{{end}}

sqlflow_secret = None
if "{{.SecretName}}" != "":
    # note(yancey1989): set dry_run to true, just reference the secret meta to generate workflow YAML,
    # we should create the secret before launching sqlflowserver
    secret_data=json.loads('''{{.SecretData}}''')
    sqlflow_secret = couler.secret(secret_data, name="{{ .SecretName }}", dry_run=True)

resources = None
if '''{{.Resources}}''' != "":
    resources=json.loads('''{{.Resources}}''')

couler.clean_workflow_after_seconds_finished({{.WorkflowTTL}})

step_log_file = "{{.StepLogFile}}"
step_exit_time_wait = {{.StepExitTimeWait}}

{{ range $ss := .StepList }}
{{.Code}}

codes = [
    "python <<EOF",
    pyfunc.body(step_entry_{{.StepIndex}}),
    "EOF",
]

if step_log_file:
    log_dir = path.dirname(step_log_file)
    codes = [
        "if [[ -f /opt/sqlflow/init_step_container.sh ]]; then",
        "  bash /opt/sqlflow/init_step_container.sh",
        "fi",
        "mkdir -p %s" % log_dir,
        "set -o pipefail # fail when any sub-command fail",
        "(",
    ] + codes + [
        ") 2>&1 | tee %s" % step_log_file,
        "exit_code=$?",
        "# sleep a while for finishing log collection",
        "sleep %d" % step_exit_time_wait,
        "exit $exit_code",
    ]

couler.run_script(image="{{.Image}}", command="bash", source="\n".join(codes), env=step_envs, resources=resources)
{{end}}
`

var coulerTemplate = template.Must(template.New("Couler").Parse(coulerCodeTmpl))
