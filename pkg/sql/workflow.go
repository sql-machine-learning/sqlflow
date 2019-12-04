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

package sql

import (
	"fmt"
	"os/exec"
	"time"
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

func fetchWorkflowLog(job WorkflowJob) error {
	fmt.Println(job.JobID)

	for i := 0; i < 10; i++ {
		cmd := exec.Command("kubectl", "get", "wf", job.JobID, "-o", "jsonpath='{.status.phase}'")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("submit Argo YAML error: %v\n%v", string(output), err)
		}

		fmt.Println(i, string(output))
		time.Sleep(time.Second)
		// Get Pod names
		_ = `kubectl get pods --selector=workflows.argoproj.io/workflow=sqlflow-couler898061205-xppzp -o jsonpath="{.items[0].metadata.name}"`
		// Get container logs
		_ = `kubectl logs sqlflow-couler898061205-xppzp-246701932 main`
		_ = `kubectl logs sqlflow-couler898061205-xppzp-246701932 wait`
	}

	return nil
}
