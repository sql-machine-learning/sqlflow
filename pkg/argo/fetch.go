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
	"os/exec"
	"regexp"
	"strings"
	"time"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

// Fetch fetches the workflow log and status
//
// if token.step_id == "" {
//    my_step := first step
// }
// logs := fetch my_step log
//
// if finish fetch my_step logs:
//    my_step = next(token.step_id)
//
// return (logs, my_step_id)
func Fetch(token pb.FetchToken) (*pb.FetchResponse, error) {
	wf, err := getWorkflowResource(token)
	if err != nil {
		return nil, err
	}
	if wf.Status.Phase == wfv1.NodePending {
		// return empty response
		return &pb.FetchResponse{
			NewToken: &pb.FetchToken{
				Job:       token.Job,
				StepId:    "",
				LogOffset: "",
				NoMoreLog: false,
			},
			Logs:  &pb.FetchResponse_Logs{},
			Phase: translatePhase(wf.Status.Phase)}, nil
	}

	stepGroupName, err := getCurrentStepGroup(wf, token)
	if err != nil {
		return nil, err
	}

	podName, err := getCurrentPodName(wf, token)
	if err != nil {
		return nil, err
	}

	pod, err := getPodResource(podName)
	if err != nil {
		return nil, err
	}

	logContent, logOffset, err := getPodLogs(pod.Name, token.GetLogOffset())
	if err != nil {
		return nil, err
	}

	finishedFetchingCurrentPod := false
	if logOffset == token.GetLogOffset() && isCompletedPhasePod(pod.Status.Phase) {
		finishedFetchingCurrentPod = true
	}

	noMoreLog := false
	newStepGroupName := ""
	if finishedFetchingCurrentPod {
		nextStepGroupName, err := getNextStepGroup(wf, token.Job.Id)
		if err != nil {
			return nil, err
		}
		// is no next step group, tag no more logs to
		if nextStepGroupName == "" {
			noMoreLog = true
		}
	} else {
		newStepGroupName = stepGroupName
	}

	return &pb.FetchResponse{
		NewToken: &pb.FetchToken{
			Job:       token.Job,
			StepId:    newStepGroupName,
			LogOffset: logOffset,
			NoMoreLog: noMoreLog},
		Logs:  &pb.FetchResponse_Logs{Content: logContent},
		Phase: translatePhase(wf.Status.Phase)}, nil
}

// NewFetchToken creates a fetch token
func NewFetchToken(job pb.Job) pb.FetchToken {
	return pb.FetchToken{
		Job:       &job,
		StepId:    "",
		LogOffset: "",
		NoMoreLog: false}
}

func getWorkflowResource(token pb.FetchToken) (*wfv1.Workflow, error) {
	cmd := exec.Command("kubectl", "get", "wf", token.Job.Id, "-o", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("getWorkflowResource error: %v\n%v", string(output), err)
	}
	return parseWorkflowResource(output)
}

func getPodResource(podName string) (*corev1.Pod, error) {
	cmd := exec.Command("kubectl", "get", "pod", podName, "-o", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed %s, %v", cmd, err)
	}
	return parsePodResource(output)
}

func checkNodeType(expected, actual wfv1.NodeType) error {
	if expected != actual {
		return fmt.Errorf("checkNodeType failed %v(expected) != %v(actual)", expected, actual)
	}
	return nil
}

func parseOffset(content string) (string, string, error) {
	reTimestamps := regexp.MustCompile(`([^\s]+)\s(.*)$`)
	msg := reTimestamps.FindStringSubmatch(content)
	if len(msg) != 3 {
		return "", "", fmt.Errorf("Parse offset failed: %s", content)
	}
	return msg[1], msg[2], nil
}

func getOffsetAndContentFromLogs(logs, oldOffset string) ([]string, string, error) {
	// NOTE(yancey1989): using `kubectl --since-time <offset>` to get logs
	// from the offset which is the timestamps. the accuracy can only be achieved at the second level,
	// `kubectl` may return some duplicated logs as the provious fetch, and we need to skip them.
	buffer := []string{}
	msgLines := strings.Split(strings.TrimSpace(logs), "\n")
	skipOlderLogs := false
	offset := oldOffset
	for _, msg := range msgLines {
		newOffset, content, e := parseOffset(msg)
		if e != nil {
			break
		}
		if newOffset == oldOffset {
			skipOlderLogs = true
		} else {
			if skipOlderLogs || oldOffset == "" {
				buffer = append(buffer, content)
				offset = newOffset
			} else {
				// skip the duplicated logs as the provious fetch
				continue
			}
		}
	}
	return buffer, offset, nil
}

func getPodLogs(podName string, offset string) ([]string, string, error) {
	// NOTE(tony): A workflow pod usually contains two container: main and wait
	// I believe wait is used for management by Argo, so we only need to care about main.
	cmd := exec.Command("kubectl", "logs", podName, "main", "--timestamps=true", fmt.Sprintf("--since-time=%s", offset))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, "", fmt.Errorf("getPodLogs error: %v\n%v", string(output), err)
	}

	return getOffsetAndContentFromLogs(string(output), offset)
}

func waitUntilComplete(token pb.FetchToken) (wf *wfv1.Workflow, err error) {
	for {
		wf, err = getWorkflowResource(token)
		if err != nil {
			return nil, fmt.Errorf("waitUntilComplete: %v", err)
		}
		if isCompletedPhaseWF(wf.Status.Phase) {
			return wf, nil
		}
		time.Sleep(time.Second)
	}
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

func getCurrentStepGroup(wf *wfv1.Workflow, token pb.FetchToken) (string, error) {
	if token.StepId == "" {
		stepNode := wf.Status.Nodes[token.Job.Id]
		if err := checkNodeType(wfv1.NodeTypeSteps, stepNode.Type); err != nil {
			return "", fmt.Errorf("getCurrentStepGroup: %v", err)
		}
		if l := len(stepNode.Children); l != 1 {
			return "", fmt.Errorf("getCurrentStepGroup: unexpected len(stepNode.Children) 1 != %v", l)
		}
		return stepNode.Children[0], nil
	}

	stepGroupNode := wf.Status.Nodes[token.StepId]
	if err := checkNodeType(wfv1.NodeTypeStepGroup, stepGroupNode.Type); err != nil {
		return "", fmt.Errorf("getNextStepGroup: %v", err)
	}
	if l := len(stepGroupNode.Children); l != 1 {
		return "", fmt.Errorf("getNextStepGroup: unexpected len(stepGroupNode.Children) 1 != %v", l)
	}
	return stepGroupNode.Name, nil
}

func getCurrentPodName(wf *wfv1.Workflow, token pb.FetchToken) (string, error) {
	if err := checkNodeType(wfv1.NodeTypeSteps, wf.Status.Nodes[token.Job.Id].Type); err != nil {
		return "", fmt.Errorf("getPodNameByStepId error: %v", err)
	}

	stepGroupName, err := getCurrentStepGroup(wf, token)
	if err != nil {
		return "", err
	}
	if stepGroupName == "" {
		return "", nil
	}

	return getPodNameByStepGroup(wf, stepGroupName)
}
