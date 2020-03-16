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
	"encoding/json"
	"fmt"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// Workflow operates Argo workflow
type Workflow struct{}

func parseWorkflowResource(b []byte) (*wfv1.Workflow, error) {
	wf := wfv1.Workflow{}
	return &wf, json.Unmarshal(b, &wf)
}

func parsePodResource(b []byte) (*corev1.Pod, error) {
	wf := corev1.Pod{}
	return &wf, json.Unmarshal(b, &wf)
}

func isPodCompleted(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded ||
		pod.Status.Phase == corev1.PodFailed
}

func isPodPending(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodPending
}

func isPodFailed(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodFailed
}

func isWorkflowPending(wf *wfv1.Workflow) bool {
	return wf.Status.Phase == wfv1.NodePending
}

func getPodNameByStepGroup(wf *wfv1.Workflow, stepGroupName string) (string, error) {
	stepGroupNode, ok := wf.Status.Nodes[stepGroupName]
	if !ok {
		return "", fmt.Errorf("getPodNameByStepGroup: stepGroup %v doesn't exist", stepGroupName)
	}
	if err := checkNodeType(wfv1.NodeTypeStepGroup, stepGroupNode.Type); err != nil {
		return "", fmt.Errorf("getPodNameByStepGroup: %v", err)
	}
	if l := len(stepGroupNode.Children); l != 1 {
		return "", fmt.Errorf("getPodNameByStepGroup: unexpected len(stepGroupNode.Children) 1 != %v", l)
	}
	return stepGroupNode.Children[0], nil
}

func getPodByStepGroup(wf *wfv1.Workflow, stepGroupName string) (*corev1.Pod, error) {
	podName, err := getPodNameByStepGroup(wf, stepGroupName)
	if err != nil {
		return nil, err
	}
	return k8sReadPod(podName)
}

func getNextStepGroup(wf *wfv1.Workflow, current string) (string, error) {
	stepGroupNode := wf.Status.Nodes[current]
	if err := checkNodeType(wfv1.NodeTypeStepGroup, stepGroupNode.Type); err != nil {
		return "", fmt.Errorf("getNextStepGroup: %v", err)
	}
	if l := len(stepGroupNode.Children); l != 1 {
		return "", fmt.Errorf("getNextStepGroup: unexpected len(stepGroupNode.Children) 1 != %v", l)
	}
	podNode := wf.Status.Nodes[stepGroupNode.Children[0]]
	if err := checkNodeType(wfv1.NodeTypePod, podNode.Type); err != nil {
		return "", fmt.Errorf("getNextStepGroup %v", err)
	}

	if len(podNode.Children) == 0 {
		return "", nil
	}
	if l := len(podNode.Children); l != 1 {
		return "", fmt.Errorf("getNextStepGroup: unexpected len(podNode.Children) 1 != %v", l)
	}
	return podNode.Children[0], nil
}

func getFirstStepGroup(wf *wfv1.Workflow, workflowID string) (string, error) {
	stepNode := wf.Status.Nodes[workflowID]
	if err := checkNodeType(wfv1.NodeTypeSteps, stepNode.Type); err != nil {
		return "", fmt.Errorf("getCurrentStepGroup: %v", err)
	}
	if l := len(stepNode.Children); l != 1 {
		return "", fmt.Errorf("getCurrentStepGroup: unexpected len(stepNode.Children) 1 != %v", l)
	}
	return stepNode.Children[0], nil
}

func getStepGroup(wf *wfv1.Workflow, workflowID, stepID string) (string, error) {
	if stepID == "" {
		return getFirstStepGroup(wf, workflowID)
	}
	return stepID, nil
}

func checkNodeType(expected, actual wfv1.NodeType) error {
	if expected != actual {
		return fmt.Errorf("checkNodeType failed %v(expected) != %v(actual)", expected, actual)
	}
	return nil
}
