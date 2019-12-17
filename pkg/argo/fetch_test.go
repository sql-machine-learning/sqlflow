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
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	pb "sqlflow.org/sqlflow/pkg/proto"
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
            value: "hello3"

  - name: whalesay
    inputs:
      parameters:
      - name: message
    container:
      image: docker/whalesay
      command: [echo]
      args: ["{{inputs.parameters.message}}"]
`

	podYAML = `apiVersion: v1
kind: Pod
metadata:
  generateName: sqlflow-pod-
spec:
  restartPolicy: Never
  containers:
  - name: main 
    image: docker/whalesay
    command: [bash]
    args: [-c, "echo 'hello1\nhello2'; sleep 2; echo 'hello3'"]
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

func kubectlDeletePod(podID string) error {
	cmd := exec.Command("kubectl", "delete", "pod", podID, "--ignore-not-found")
	_, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed %s, %v", cmd, err)
	}
	return nil
}

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

func TestFetch(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST") != "workflow" {
		t.Skip("argo: skip workflow tests")
	}
	a := assert.New(t)
	workflowID, err := kubectlCreateFromYAML(stepYAML)
	a.NoError(err)

	token := NewFetchToken(pb.Job{Id: workflowID})
	actualLogs := []string{}
	for {
		response, err := Fetch(token)
		a.NoError(err)
		for _, log := range response.Logs.Content {
			actualLogs = append(actualLogs, log)
		}
		if isCompletePhasePB(response.Phase) && response.NewToken.NoMoreLog {
			break
		}
		time.Sleep(time.Second)
		token = *response.NewToken
	}

	expectedLogs := []string{"hello1", "hello2", "hello3"}
	a.Equal(len(expectedLogs), len(actualLogs))
	for i := range expectedLogs {
		a.Equal(expectedLogs[i], actualLogs[i])
	}
}

func waitUntilPodRunning(podID string) error {
	for {
		cmd := exec.Command("kubectl", "get", "pod", podID, "-o", "jsonpath={.status.phase}")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return err
		}
		if string(output) != "Pending" {
			break
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}

func isPodCompleted(podID string) bool {
	cmd := exec.Command("kubectl", "get", "pod", podID, "-o", "jsonpath={.status.phase}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	if string(output) == "Succeeded" {
		return true
	}
	return false
}

func TestGetPodLogs(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST") != "workflow" {
		t.Skip("argo: skip workflow tests")
	}
	a := assert.New(t)
	podID, err := kubectlCreateFromYAML(podYAML)
	a.NoError(err)
	defer kubectlDeletePod(podID)

	err = waitUntilPodRunning(podID)
	a.NoError(err)
	offset := ""
	actual := []string{}
	expected := []string{"hello1", "hello2", "hello3"}
	for {
		isPodCompleted := isPodCompleted(podID)
		logs, newOffset, err := getPodLogs(podID, offset)
		a.NoError(err)
		if len(logs) != 0 {
			actual = append(actual, logs...)
		}
		if isPodCompleted && offset == newOffset {
			break
		}
		offset = newOffset
		time.Sleep(1 * time.Second)
	}
	a.Equal(expected, actual)

}
