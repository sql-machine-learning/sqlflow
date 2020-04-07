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

package tekton

import (
	"fmt"
	"os"
	"strings"

	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// Workflow parses Workflow TaskRun and Pod object and returns
// the workflow step status
type Workflow struct {
	pod     *corev1.Pod
	stepID  string
	taskrun *tektonapi.TaskRun
}

func (w *Workflow) isPending() bool {
	return w.taskrun.Status.Conditions[0].Reason == "Pending"
}

func (w *Workflow) stepLogURL() string {
	ep := os.Getenv("SQLFLOW_WORKFLOW_LOGVIEW_ENDPOINT")
	return fmt.Sprintf("%s/proxy/api/v1/namespaces/%s/pods/%s/log?container=%s", ep, w.c.namespace, w.pod.ObjectMeta.Name, w.stepID)
}

func (w *Workflow) stepCode() (string, error) {
	_, stepIdx, e := w.stepIdx()
	if e != nil {
		return "", e
	}
	return fmt.Sprintf("%s %s", strings.Join(w.pod.Spec.Containers[stepIdx].Command, " "), strings.Join(w.pod.Spec.Containers[stepIdx].Args, " ")), nil
}

func (w *Workflow) stepPhase() string {
	return w.pod.Status.ContainerStatuses[0].State.String()
}

func (w *Workflow) nextStepID() string {
	return ""
}

func (w *Workflow) stepLogs() ([]string, error) {
	return []string{}, nil
}
func (w *Workflow) isStepComplete() bool {
	return false
}

func (w *Workflow) isStepFailed() bool {
	return false
}

func (w *Workflow) creationSecond() int {
	return 1
}

func (w *Workflow) stepIdx() (int, int, error) {
	cnt := len(w.taskrun.Status.Steps)
	for i, step := range w.taskrun.Status.Steps {
		if step.Name == w.stepID {
			return cnt, i + 1, nil
		}
	}
	return -1, -1, fmt.Errorf("can not find the step index of: %s", w.stepID)
}

func newWorkflow(workflowID, stepID string) (*Workflow, error) {
	w := &Workflow{}
	c, e := newClient()
	if e != nil {
		return nil, e
	}
	taskrun, e := c.getTaskRun(workflowID)
	if e != nil {
		return nil, e
	}
	pod, e := c.getPod(w.taskrun.Status.PodName)
	if e != nil {
		return nil, e
	}
	if stepID == "" {
		stepID = w.taskrun.Status.Steps[0].Name
	}
	return &Workflow{
		pod:     pod,
		taskrun: taskrun,
		stepID:  stepID,
	}, nil
}
