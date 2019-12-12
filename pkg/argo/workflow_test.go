// Copyright 2019 The SQLFlow Authors. All rights reserved.
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

package argo

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"os/exec"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"testing"
)

const (
	stepYAML = `apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: steps-
spec:
  entrypoint: hello-hello-hello
  templates:
  - name: hello-hello-hello
    steps:
    - - name: hello1
        template: whalesay
        arguments:
          parameters:
          - name: message
            value: "hello1"
    - - name: hello2
        template: whalesay
        arguments:
          parameters:
          - name: message
            value: "hello2"
    - - name: hello3
        template: whalesay
        arguments:
          parameters:
          - name: message
            value: "hello1"

  - name: whalesay
    inputs:
      parameters:
      - name: message
    container:
      image: docker/whalesay
      command: [echo]
      args: ["{{inputs.parameters.message}}"]
`
)

func createAndWriteTempFile(content string) (string, error) {
	tmpFile, err := ioutil.TempFile("/tmp", "sqlflow-")
	if err != nil {
		return "", nil
	}
	defer tmpFile.Close()

	if _, err = tmpFile.Write([]byte(content)); err != nil {
		return "", err
	}

	return tmpFile.Name(), nil
}

func kubectlCreateFromYAML(content string) (string, error) {
	fileName, err := createAndWriteTempFile(content)
	if err != nil {
		return "", err
	}
	defer os.Remove(fileName)

	cmd := exec.Command("kubectl", "create", "-f", fileName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("submitYAML error: %v\n%v", string(output), err)
	}

	return getWorkflowID(string(output))
}

//
//func TestGetStepPodNames(t *testing.T) {
//	if os.Getenv("SQLFLOW_TEST") != "workflow" {
//		t.Skip("argo: skip workflow tests")
//	}
//	a := assert.New(t)
//	workflowID, err := kubectlCreateFromYAML(stepYAML)
//	a.NoError(err)
//	err = waitUntilComplete(pb.Job{Id: workflowID})
//	a.NoError(err)
//	wf, err := getWorkflowResource(pb.Job{Id: workflowID})
//	a.NoError(err)
//	podNames, err := getStepPodNames(wf.Status.Nodes, pb.Job{Id: workflowID})
//	a.NoError(err)
//	a.Equal(3, len(podNames))
//}

func TestGetCurrentStepGroup(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST") != "workflow" {
		t.Skip("argo: skip workflow tests")
	}
	a := assert.New(t)
	output := []byte(testWorkflowDescription)
	wf, err := parseWorkflowResource(output)
	a.NoError(err)

	stepGroupNames := []string{
		"",
		"steps-7lxxs-1184503397",
		"steps-7lxxs-43875568",
		"steps-7lxxs-43331115",
		""}
	for i := 0; i < len(stepGroupNames)-1; i++ {
		currentStepGroup, err := getCurrentStepGroup(wf, pb.FetchToken{Job: &pb.Job{Id: "steps-7lxxs"}, StepId: stepGroupNames[i]})
		a.NoError(err)
		a.Equal(stepGroupNames[i+1], currentStepGroup)
	}
}

func TestGetNextStepGroup(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST") != "workflow" {
		t.Skip("argo: skip workflow tests")
	}
	a := assert.New(t)
	output := []byte(testWorkflowDescription)
	wf, err := parseWorkflowResource(output)
	a.NoError(err)

	stepGroupNames := []string{
		"steps-7lxxs-1184503397",
		"steps-7lxxs-43875568",
		"steps-7lxxs-43331115",
		""}
	for i := 0; i < len(stepGroupNames)-1; i++ {
		next, err := getNextStepGroup(wf, stepGroupNames[i])
		a.NoError(err)
		a.Equal(stepGroupNames[i+1], next)
	}
}

func TestGetPodNameByStepGroup(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST") != "workflow" {
		t.Skip("argo: skip workflow tests")
	}
	a := assert.New(t)
	output := []byte(testWorkflowDescription)
	wf, err := parseWorkflowResource(output)
	a.NoError(err)

	stepGroupNames := []string{
		"steps-7lxxs-1184503397",
		"steps-7lxxs-43875568",
		"steps-7lxxs-43331115"}
	podNames := []string{
		"steps-7lxxs-2267726410",
		"steps-7lxxs-1263033216",
		"steps-7lxxs-1288663778"}
	for i := 0; i < len(stepGroupNames); i++ {
		podName, err := getPodNameByStepGroup(wf, stepGroupNames[i])
		a.NoError(err)
		a.Equal(podNames[i], podName)
	}
}

func TestGetCurrentPodName(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST") != "workflow" {
		t.Skip("argo: skip workflow tests")
	}
	a := assert.New(t)
	output := []byte(testWorkflowDescription)
	wf, err := parseWorkflowResource(output)
	a.NoError(err)

	stepIds := []string{
		"",
		"steps-7lxxs-1184503397",
		"steps-7lxxs-43875568",
		"steps-7lxxs-43331115"}
	podNames := []string{
		"steps-7lxxs-2267726410",
		"steps-7lxxs-1263033216",
		"steps-7lxxs-1288663778",
		""}
	for i := 0; i < len(stepIds); i++ {
		currentPod, err := getCurrentPodName(wf, pb.FetchToken{Job: &pb.Job{Id: "steps-7lxxs"}, StepId: stepIds[i]})
		a.NoError(err)
		a.Equal(podNames[i], currentPod)
	}
}
