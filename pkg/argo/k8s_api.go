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
	"os/exec"
	"regexp"
	"strings"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func k8sCreateResource(yamlContent string) (string, error) {
	// create Kubernetes resource and fetch the resource ID
	cmd := exec.Command("kubectl", "create", "-f", "-")
	cmd.Stdin = strings.NewReader(yamlContent)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("submit Argo YAML error: %v, output: %s", err, string(output))
	}

	re := regexp.MustCompile(`.+/(.+) .+`)
	createRes := re.FindStringSubmatch(string(output))
	if len(createRes) != 2 {
		return "", fmt.Errorf("parse created resource error: %s, %v", cmd, output)
	}
	return createRes[1], nil
}

func k8sReadWorkflow(workflowID string) (*wfv1.Workflow, error) {
	cmd := exec.Command("kubectl", "get", "wf", workflowID, "-o", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("getWorkflowResource error: %v\n%v", string(output), err)
	}
	return parseWorkflowResource(output)
}

func k8sReadPod(podName string) (*corev1.Pod, error) {
	cmd := exec.Command("kubectl", "get", "pod", podName, "-o", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("cmd: %s failed: %v", cmd, err)
	}
	return parsePodResource(output)
}

func k8sReadPodLogs(podName, containerName, sinceTime string, enableTimeStamp bool) ([]string, error) {
	cmdArray := []string{"kubectl", "logs", podName, containerName}
	if enableTimeStamp {
		cmdArray = append(cmdArray, []string{"--timestamps=true", fmt.Sprintf("--since-time=%s", sinceTime)}...)
	}
	cmd := exec.Command(cmdArray[0], cmdArray[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("getPodLogs error: %v\n%v", string(output), err)
	}
	return strings.Split(strings.TrimSpace(string(output)), "\n"), nil
}

func k8sDeletePod(podID string) error {
	cmd := exec.Command("kubectl", "delete", "pod", podID, "--ignore-not-found")
	_, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed %s, %v", cmd, err)
	}
	return nil
}

func k8sDeleteWorkflow(workflowID string) error {
	cmd := exec.Command("kubectl", "delete", "workflow", workflowID, "--ignore-not-found")
	_, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed %s, %v", cmd, err)
	}
	return nil
}
