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
	"os"
	"regexp"
	"strings"

	"github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

func newFetchRequest(workflowID, stepID, logOffset string) *pb.FetchRequest {
	return &pb.FetchRequest{
		Job: &pb.Job{
			Id: workflowID,
		},
		StepId:    stepID,
		LogOffset: logOffset,
	}
}

func newFetchResponse(newReq *pb.FetchRequest, eof bool, logs []string) *pb.FetchResponse {
	return &pb.FetchResponse{
		UpdatedFetchSince: newReq,
		Eof:               eof,
		Logs: &pb.FetchResponse_Logs{
			Content: logs,
		},
	}
}

func getStepIdx(wf *v1alpha1.Workflow, targetStepGroup string) (int, error) {
	stepIdx := 1
	stepGroupName, e := getFirstStepGroup(wf, wf.ObjectMeta.Name)
	if e != nil {
		return -1, e
	}
	for {
		if stepGroupName == targetStepGroup {
			return stepIdx, nil
		}
		stepGroupName, e = getNextStepGroup(wf, stepGroupName)
		stepIdx++
		if e != nil {
			return -1, e
		}
	}
}

func logViewURL(ns, wfID, stepID string) (string, error) {
	ep := os.Getenv("SQLFLOW_ARGO_UI_ENDPOINT")
	if ep == "" {
		return "", fmt.Errorf("should set SQLFLOW_ARGO_UI_ENDPOINT if enable Argo mode")
	}
	return fmt.Sprintf("%s/workflows/%s/%s?nodeId=%s", ep, ns, wfID, stepID), nil
}

// Fetch fetches the workflow log and status,
// design doc: https://github.com/sql-machine-learning/sqlflow/blob/develop/doc/design/argo_workflow_on_sqlflow.md
func Fetch(req *pb.FetchRequest) (*pb.FetchResponse, error) {
	wf, err := k8sReadWorkflow(req.Job.Id)
	if err != nil {
		return nil, err
	}

	if isWorkflowPending(wf) {
		return newFetchResponse(req, false, []string{}), nil
	}
	stepGroupName, err := getStepGroup(wf, req.Job.Id, req.StepId)
	if err != nil {
		return nil, err
	}
	stepCnt := len(wf.Spec.Templates[0].Steps)
	stepIdx, err := getStepIdx(wf, stepGroupName)
	if err != nil {
		return nil, err
	}

	pod, err := getPodByStepGroup(wf, stepGroupName)
	if err != nil {
		return nil, err
	}
	eof := false // true if finish fetching the workflow logs
	logs := []string{}

	// An example log content:
	// Step [1/3] Execute Code:
	// repl -e "SELECT * FROM iris.train"
	// Step [1/3] Log view: http://localhost:8001/workflows/default/steps-bdpff
	// Step [1/3] Status: Pending
	// Step [1/3] Status: Running
	// Step [1/3] Status: ...
	// Step [1/3] Status: Succeed/Failed
	// ..
	newOffset := req.LogOffset
	if req.LogOffset == "" {
		// return the log view url for the first call of step
		url, e := logViewURL(wf.ObjectMeta.Namespace, wf.ObjectMeta.Name, stepGroupName)
		if e != nil {
			return nil, e
		}
		// the 1-th container execute `argoexec wait` to wait the preiority step, so package the 2-th container's command code.
		execCode := fmt.Sprintf("%s %s", strings.Join(pod.Spec.Containers[1].Command, " "), strings.Join(pod.Spec.Containers[1].Args, " "))
		logs = append(logs, fmt.Sprintf("Step: [%d/%d] Execute Code: %s", stepIdx, stepCnt, execCode))
		logs = append(logs, fmt.Sprintf("Step: [%d/%d] Log View: %s", stepIdx, stepCnt, url))
	}

	// output the refreshed status if the Pod Phase changed.
	if req.LogOffset != string(pod.Status.Phase) {
		logs = append(logs, fmt.Sprintf("Step: [%d/%d] Status: %s", stepIdx, stepCnt, pod.Status.Phase))
		newOffset = string(pod.Status.Phase)
	}

	if isPodCompleted(pod) {
		// move to the next step
		if stepGroupName, err = getNextStepGroup(wf, stepGroupName); err != nil {
			return nil, err
		}
		// set the EOF to true if no next step in the workflow
		if stepGroupName == "" {
			eof = true
		}
		newOffset = ""
		if isPodFailed(pod) {
			return newFetchResponse(newFetchRequest(req.Job.Id, stepGroupName, newOffset), eof, logs), fmt.Errorf("step failed")
		}
	}

	return newFetchResponse(newFetchRequest(req.Job.Id, stepGroupName, newOffset), eof, logs), nil
}

func parseOffset(content string) (string, string, error) {
	reTimestamps := regexp.MustCompile(`([^\s]+)\s(.*)$`)
	msg := reTimestamps.FindStringSubmatch(content)
	if len(msg) != 3 {
		return "", "", fmt.Errorf("Parse offset failed: %s", content)
	}
	return msg[1], msg[2], nil
}

func getOffsetAndContentFromLogs(logs []string, oldOffset string) ([]string, string, error) {
	// NOTE(yancey1989): using `kubectl --since-time <offset>` to get logs
	// from the offset which is the timestamps. the accuracy can only be achieved at the second level,
	// `kubectl` may return some duplicated logs as the previous fetch, and we need to skip them.
	buffer := []string{}
	skipOlderLogs := false
	offset := oldOffset
	for _, msg := range logs {
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
				// skip the duplicated logs as the previous fetch
				continue
			}
		}
	}
	return buffer, offset, nil
}

func getPodLogs(podName string, offset string) ([]string, string, error) {
	// NOTE(tony): A workflow pod usually contains two container: main and wait
	// I believe wait is used for management by Argo, so we only need to care about main.
	logs, err := k8sReadPodLogs(podName, "main", offset)
	if err != nil {
		return nil, "", err
	}

	return getOffsetAndContentFromLogs(logs, offset)
}
