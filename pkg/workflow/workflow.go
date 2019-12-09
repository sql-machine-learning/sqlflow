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

package workflow

import (
	"fmt"
	"os/exec"
	"regexp"
	"time"

	pb "sqlflow.org/sqlflow/pkg/proto"
)

// Reference: https://github.com/argoproj/argo/blob/723b3c15e55d2f8dceb86f1ac0a6dc7d1a58f10b/pkg/apis/workflow/v1alpha1/workflow_types.go#L30-L38

// NodePhase is a label for the condition of a node at the current time.
type NodePhase string

// Workflow and node statuses
const (
	NodePending   NodePhase = "Pending"
	NodeRunning   NodePhase = "Running"
	NodeSucceeded NodePhase = "Succeeded"
	NodeSkipped   NodePhase = "Skipped"
	NodeFailed    NodePhase = "Failed"
	NodeError     NodePhase = "Error"
)

func isCompletedPhase(phase NodePhase) bool {
	return phase == NodeSucceeded ||
		phase == NodeFailed ||
		phase == NodeError ||
		phase == NodeSkipped
}

func getWorkflowID(output string) (string, error) {
	reWorkflow := regexp.MustCompile(`.+/(.+) .+`)
	wf := reWorkflow.FindStringSubmatch(string(output))
	if len(wf) != 2 {
		return "", fmt.Errorf("parse workflow ID error: %v", output)
	}

	return wf[1], nil
}

func getWorkflowStatusPhase(job pb.Job) (string, error) {
	cmd := exec.Command("kubectl", "get", "wf", job.Id, "-o", "jsonpath={.status.phase}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("getWorkflowStatusPhase error: %v\n%v", string(output), err)
	}

	return string(output), nil
}

func getWorkflowPodName(job pb.Job) (string, error) {
	cmd := exec.Command("kubectl", "get", "pods",
		fmt.Sprintf(`--selector=workflows.argoproj.io/workflow=%s`, job.Id),
		"-o", "jsonpath={.items[0].metadata.name}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("getWorkflowPodName error: %v\n%v", string(output), err)
	}

	return string(output), nil
}

func getPodLogs(podName string) (string, error) {
	// NOTE(tony): A workflow pod usually contains two container: main and wait
	// I believe wait is used for management by Argo, so we only need to care about main.
	cmd := exec.Command("kubectl", "logs", podName, "main")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("getPodLogs error: %v\n%v", string(output), err)
	}
	return string(output), nil
}

func fetchWorkflowLog(job pb.Job) (string, error) {
	for {
		statusPhase, err := getWorkflowStatusPhase(job)
		if err != nil {
			return "", err
		}

		// FIXME(tony): what if it is a long running job
		if isCompletedPhase(NodePhase(statusPhase)) {
			break
		}
		time.Sleep(time.Second)
	}

	// FIXME(tony): what if there are multiple pods
	podName, err := getWorkflowPodName(job)
	if err != nil {
		return "", err
	}

	return getPodLogs(podName)
}
