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

package argo

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/stretchr/testify/assert"
	sqlflowlog "sqlflow.org/sqlflow/go/log"
	pb "sqlflow.org/sqlflow/go/proto"
	wfrsp "sqlflow.org/sqlflow/go/workflow/response"
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
        template: query
        arguments:
          parameters:
          - name: sql 
            value: "select 1;"
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
      image: %s
      command: [echo]
      args: ["{{inputs.parameters.message}}"]
  - name: query
    inputs:
      parameters:
      - name: sql
    container:
      env:
      - name: SQLFLOW_MYSQL_HOST
        value: '0.0.0.0'
      - name: SQLFLOW_MYSQL_PORT
        value: '3306'
      - name: SQLFLOW_DATASOURCE
        value: '%s'
      image: %s
      command: [bash]
      args: ["-c", 'bash /start.sh mysql >/dev/null 2>&1 & step -e "{{inputs.parameters.sql}}"']
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

	podYAML2 = `apiVersion: v1
kind: Pod
metadata:
  generateName: sqlflow-pod-
spec:
  restartPolicy: Never
  containers:
  - name: main
    image: docker/whalesay
    command: [bash]
    args: [-c, "for i in {0..1000}; do   echo $i;   sleep 0.00$((RANDOM % 100)); done"]
`
)

var stepImage = "sqlflow/sqlflow"

func init() {
	sqlflowlog.InitLogger("/dev/null", sqlflowlog.TextFormatter)
}

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

func parseFetchResponse(responses *pb.FetchResponse_Responses) ([]string, [][]*any.Any, []string, error) {
	columns := []string{}
	rows := [][]*any.Any{}
	messages := []string{}
	for _, res := range responses.GetResponse() {
		switch res.GetResponse().(type) {
		case *pb.Response_Message:
			messages = append(messages, res.GetMessage().Message)
		case *pb.Response_Head:
			response := &pb.Response{}
			if e := proto.UnmarshalText(res.String(), response); e != nil {
				return nil, nil, nil, e
			}
			columns = append(columns, response.GetHead().GetColumnNames()...)
		case *pb.Response_Row:
			response := &pb.Response{}
			if e := proto.UnmarshalText(res.String(), response); e != nil {
				return nil, nil, nil, e
			}
			rows = append(rows, response.GetRow().GetData())
		default:
			// continue
		}
	}
	return columns, rows, messages, nil
}

func TestFetch(t *testing.T) {
	a := assert.New(t)
	if os.Getenv("SQLFLOW_TEST") != "workflow" {
		t.Skip("argo: skip workflow tests")
	}
	os.Setenv("SQLFLOW_WORKFLOW_LOGVIEW_ENDPOINT", "http://localhost:8001")
	defer os.Unsetenv("SQLFLOW_WORKFLOW_LOGVIEW_ENDPOINT")

	if os.Getenv("SQLFLOW_WORKFLOW_STEP_IMAGE") != "" {
		stepImage = os.Getenv("SQLFLOW_WORKFLOW_STEP_IMAGE")
	}
	ds := os.Getenv("SQLFLOW_TEST_DATASOURCE")
	a.NotEmpty(ds, fmt.Sprintf("should set SQLFLOW_TEST_DATASOURCE env"))

	workflowID, err := k8sCreateResource(fmt.Sprintf(stepYAML, stepImage, ds, stepImage))
	a.NoError(err)

	defer k8sDeleteWorkflow(workflowID)
	req := wfrsp.NewFetchRequest(workflowID, "", "")
	wf := &Workflow{}
	fr, err := wf.Fetch(req)
	messages := []string{}
	columns := []string{}
	rows := [][]*any.Any{}
	for {
		if err != nil {
			break
		}
		// process Response protobuf message
		c, r, m, e := parseFetchResponse(fr.GetResponses())
		if e != nil {
			err = e
			break
		}
		if len(c) > 0 {
			columns = c
			rows = r
		}
		messages = append(messages, m...)
		if fr.Eof {
			break
		}
		time.Sleep(time.Second)
		fr, err = wf.Fetch(fr.UpdatedFetchSince)
	}
	a.NoError(err)

	concatedLogs := strings.Join(messages, "\n")

	a.Contains(concatedLogs, "SQLFlow Step: [1/3] Status: Succeeded")
	a.Contains(concatedLogs, "SQLFlow Step: [2/3] Status: Succeeded")
	a.Contains(concatedLogs, "SQLFlow Step: [3/3] Status: Succeeded")
	// confirm columns and rows of sql: SELECT 1;
	a.Equal([]string{"1"}, columns)
	v := &wrappers.Int32Value{}
	a.NoError(ptypes.UnmarshalAny(rows[0][0], v))
	a.Equal(v.GetValue(), int32(1))
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

func TestGetPodLogs(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST") != "workflow" {
		t.Skip("argo: skip workflow tests")
	}
	a := assert.New(t)
	podID, err := k8sCreateResource(podYAML)
	a.NoError(err)
	defer k8sDeletePod(podID)

	err = waitUntilPodRunning(podID)
	a.NoError(err)
	offset := ""
	actual := []string{}
	expected := []string{"hello1", "hello2", "hello3"}
	for {
		pod, err := k8sReadPod(podID)
		a.NoError(err)
		isPodCompleted := isPodCompleted(pod)
		logs, newOffset, err := getPodLogs(pod.Name, offset)
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

func TestGetPodLogsStress(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST") != "workflow" {
		t.Skip("argo: skip workflow tests")
	}
	a := assert.New(t)
	podID, err := k8sCreateResource(podYAML2)
	a.NoError(err)
	defer k8sDeletePod(podID)

	err = waitUntilPodRunning(podID)
	a.NoError(err)
	offset := ""
	actual := []string{}
	for {
		pod, err := k8sReadPod(podID)
		a.NoError(err)
		isPodCompleted := isPodCompleted(pod)
		logs, newOffset, err := getPodLogs(pod.Name, offset)
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
	expected := []string{}
	for i := 0; i <= 1000; i++ {
		expected = append(expected, strconv.FormatInt(int64(i), 10))
	}
	a.Equal(expected, actual)
}
