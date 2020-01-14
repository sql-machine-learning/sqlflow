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
	"regexp"

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

	pod, err := getPodByStepGroup(wf, stepGroupName)
	if err != nil {
		return nil, err
	}

	if isPodPending(pod) {
		return newFetchResponse(req, false, []string{}), nil
	}

	logs, newLogOffset, err := getPodLogs(pod.Name, req.LogOffset)
	if err != nil {
		return nil, err
	}

	finishFetchingCurrentPod := false // true if finish fetching the current pod logs
	eof := false                      // true if finish fetching the workflow logs

	// finish fetching the current pod logs when:
	// 1. the Pod is completed(succeed/failed/error).
	// 2. no updated logs.
	if req.LogOffset == newLogOffset && isPodCompleted(pod) {
		finishFetchingCurrentPod = true
	}

	if finishFetchingCurrentPod {
		if stepGroupName, err = getNextStepGroup(wf, stepGroupName); err != nil {
			return nil, err
		}
		newLogOffset = ""

		// set the EOF to true if no next step in the workflow
		if stepGroupName == "" {
			eof = true
		}

		if isPodFailed(pod) {
			return newFetchResponse(newFetchRequest(req.Job.Id, stepGroupName, newLogOffset), eof, logs), fmt.Errorf("workflow step failed")
		}
	}

	return newFetchResponse(newFetchRequest(req.Job.Id, stepGroupName, newLogOffset), eof, logs), nil
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
