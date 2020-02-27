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
	"github.com/gogo/protobuf/proto"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

func newFetchRequest(workflowID, stepID, stepPhase string) *pb.FetchRequest {
	return &pb.FetchRequest{
		Job: &pb.Job{
			Id: workflowID,
		},
		StepId:    stepID,
		StepPhase: stepPhase,
	}
}

func newFetchResponse(newReq *pb.FetchRequest, eof bool, responses []*pb.Response) *pb.FetchResponse {
	return &pb.FetchResponse{
		UpdatedFetchSince: newReq,
		Eof:               eof,
		Responses: &pb.FetchResponse_Responses{
			Response: responses,
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

func logViewURL(ns, wfID, podID string) (string, error) {
	ep := os.Getenv("SQLFLOW_ARGO_UI_ENDPOINT")
	// argo UI log view panel
	return fmt.Sprintf("%s/workflows/%s/%s?tab=workflow&nodeId=%s&sidePanel=logs:%s:main", ep, ns, wfID, podID, podID), nil
}

// Fetch fetches the workflow log and status,
// design doc: https://github.com/sql-machine-learning/sqlflow/blob/develop/doc/design/argo_workflow_on_sqlflow.md
func Fetch(req *pb.FetchRequest) (*pb.FetchResponse, error) {
	wf, err := k8sReadWorkflow(req.Job.Id)
	if err != nil {
		return nil, err
	}

	if isWorkflowPending(wf) {
		return newFetchResponse(req, false, []*pb.Response{}), nil
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
	// Step [1/3] Execute Code: echo hello1
	// Step [1/3] Log: http://localhost:8001/workflows/default/steps-bdpff?nodeId=steps-bdpff-xx1
	// Step [1/3] Status: Pending
	// Step [1/3] Status: Running
	// Step [1/3] Status: Succeed/Failed
	// Step [2/3] Execute Code: echo hello2
	// Step [2/3] Log: http://localhost:8001/workflows/default/steps-bdpff?nodeId=steps-bdpff-xx2
	// ...
	newStepPhase := req.StepPhase
	logURL, e := logViewURL(wf.ObjectMeta.Namespace, wf.ObjectMeta.Name, pod.ObjectMeta.Name)
	if e != nil {
		return nil, e
	}

	if req.StepPhase == "" {
		// the 1-th container execute `argoexec wait` to wait the preiority step, so package the 2-th container's command code.
		execCode := fmt.Sprintf("%s %s", strings.Join(pod.Spec.Containers[1].Command, " "), strings.Join(pod.Spec.Containers[1].Args, " "))
		logs = append(logs, fmt.Sprintf("SQLFlow Step: [%d/%d] Execute Code: %s", stepIdx, stepCnt, execCode))
		logs = append(logs, fmt.Sprintf("SQLFlow Step: [%d/%d] Log: %s", stepIdx, stepCnt, logURL))
	}

	// note(yancey1989): record the Pod phase to avoid output the duplicated logs at the next fetch request.
	if req.StepPhase != string(pod.Status.Phase) {
		logs = append(logs, fmt.Sprintf("SQLFlow Step: [%d/%d] Status: %s", stepIdx, stepCnt, pod.Status.Phase))
		newStepPhase = string(pod.Status.Phase)
	}

	responses := []*pb.Response{}
	for _, log := range logs {
		r, e := pb.EncodeMessage(log)
		if e != nil {
			return nil, e
		}
		responses = append(responses, r)
	}

	if isPodCompleted(pod) {
		if isPodFailed(pod) {
			return newFetchResponse(newFetchRequest(req.Job.Id, stepGroupName, newStepPhase), eof, responses),
				fmt.Errorf("SQLFlow Step [%d/%d] Failed, Log: %s", stepIdx, stepCnt, logURL)
		}

		// snip the pod logs when it complete
		// TODO(yancey1989): fetch the pod logs using an iteration way
		// to avoid the memory overflow
		podLogs, e := k8sReadPodLogs(pod.ObjectMeta.Name, "main", "", false)
		if e != nil {
			return nil, e
		}

		// unmarshl pb.Response from pod logs
		r, e := unMarshalPodLogs(podLogs)
		if e != nil {
			return nil, e
		}
		responses = append(responses, r...)

		// move to the next step
		nextStepGroup, err := getNextStepGroup(wf, stepGroupName)
		if err != nil {
			return nil, err
		}
		// set the EOF to true if no next step in the workflow
		if nextStepGroup == "" && stepIdx == stepCnt {
			eof = true
		}

		if nextStepGroup != "" {
			newStepPhase = ""
			stepGroupName = nextStepGroup
		}
	}

	return newFetchResponse(newFetchRequest(req.Job.Id, stepGroupName, newStepPhase), eof, responses), nil
}

func isHTMLCode(code string) bool {
	//TODO(yancey1989): support more lines HTML code e.g.
	//<div>
	//  ...
	//</div>
	re := regexp.MustCompile(`<div.*?>.*</div>`)
	return re.MatchString(code)
}

func unMarshalPodLogs(podLogs []string) ([]*pb.Response, error) {
	responses := []*pb.Response{}
	for _, log := range podLogs {
		if isHTMLCode(log) {
			r, e := pb.EncodeMessage(log)
			if e != nil {
				return nil, e
			}
			responses = append(responses, r)
		}
		response := &pb.Response{}
		if e := proto.UnmarshalText(log, response); e != nil {
			// skip this line if it's not protobuf message
			continue
		}
		responses = append(responses, response)
	}
	return responses, nil
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
	logs, err := k8sReadPodLogs(podName, "main", offset, true)
	if err != nil {
		return nil, "", err
	}

	return getOffsetAndContentFromLogs(logs, offset)
}
