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
	"time"

	"github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	"sqlflow.org/sqlflow/go/log"
	pb "sqlflow.org/sqlflow/go/proto"
	wfrsp "sqlflow.org/sqlflow/go/workflow/response"
)

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
	// if we are using DTM log collection, construct a different url pattern
	dtmEp := os.Getenv("SQLFLOW_WORKFLOW_LOGVIEW_DTM_ENDPOINT")
	if dtmEp != "" {
		// Set time interval to 24 hour for the first page, users can adjust
		// this time on logview page
		startTime, endTime := time.Now().Unix()*1000, time.Now().Add(24*time.Hour).Unix()*1000
		return fmt.Sprintf("%s?jobName=%s&taskName=%s&__envName=PROD&startTime=%d&endTime=%d",
			dtmEp, wfID, podID, startTime, endTime), nil
	}
	// argo UI log view panel
	ep := os.Getenv("SQLFLOW_WORKFLOW_LOGVIEW_ENDPOINT")
	return fmt.Sprintf("%s/workflows/%s/%s?tab=workflow&nodeId=%s&sidePanel=logs:%s:main", ep, ns, wfID, podID, podID), nil
}

// Fetch fetches the workflow log and status,
// design doc: https://github.com/sql-machine-learning/sqlflow/blob/develop/doc/design/argo_workflow_on_sqlflow.md
//
// An example of Fetched Responses from the server logs:
// Step [1/3] Execute Code: echo hello1
// Step [1/3] Log: http://localhost:8001/workflows/default/steps-bdpff?nodeId=steps-bdpff-xx1
// Step [1/3] Status: Pending
// Step [1/3] Status: Running
// Step [1/3] Status: Succeed/Failed
// Step [2/3] Execute Code: echo hello2
// Step [2/3] Log: http://localhost:8001/workflows/default/steps-bdpff?nodeId=steps-bdpff-xx2
// ...
func (w *Workflow) Fetch(req *pb.FetchRequest) (*pb.FetchResponse, error) {
	logger := log.WithFields(log.Fields{
		"requestID": log.UUID(),
		"jobID":     req.Job.Id,
		"stepID":    req.StepId,
		"event":     "fetch",
	})

	wf, err := k8sReadWorkflow(req.Job.Id)
	if err != nil {
		logger.Errorf("workflowFailed/k8sRead, error: %v", err)
		return nil, err
	}
	logger.Infof("phase:%s", wf.Status.Phase)

	if isWorkflowPending(wf) {
		return wfrsp.New().Response(req.Job.Id, "", "", false), nil
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
	logPrefix := fmt.Sprintf("SQLFlow Step: [%d/%d]", stepIdx, stepCnt)

	pod, err := getPodByStepGroup(wf, stepGroupName)
	if err != nil {
		return nil, err
	}
	eof := false // true if finish fetching the workflow logs
	r := wfrsp.New()
	newStepPhase := req.StepPhase
	logURL, e := logViewURL(wf.ObjectMeta.Namespace, wf.ObjectMeta.Name, pod.ObjectMeta.Name)
	if e != nil {
		return nil, e
	}

	if req.StepPhase == "" {
		// the 1st container execute `argoexec wait` to wait the priority step, so package the 2nd container's command code.
		execCode := fmt.Sprintf("%s %s", strings.Join(pod.Spec.Containers[1].Command, " "), strings.Join(pod.Spec.Containers[1].Args, " "))
		r.AppendMessage(fmt.Sprintf("%s Execute Code: %s", logPrefix, execCode))
		r.AppendMessage(fmt.Sprintf("%s Log: %s", logPrefix, logURL))
	}

	// note(yancey1989): record the Pod phase to avoid output the duplicated logs at the next fetch request.
	if req.StepPhase != string(pod.Status.Phase) {
		r.AppendMessage(fmt.Sprintf("%s Status: %s", logPrefix, pod.Status.Phase))
		newStepPhase = string(pod.Status.Phase)
	}

	if isPodCompleted(pod) {
		// snip the pod logs when it complete
		// TODO(yancey1989): fetch the pod logs using an iteration way
		// to avoid the memory overflow
		podLogs, e := k8sReadPodLogs(pod.ObjectMeta.Name, "main", "", false)
		if e != nil {
			return nil, e
		}

		if e := r.AppendProtoMessages(podLogs); e != nil {
			return nil, e
		}

		// TODO(yancey1989): add duration time for the eoeResponse
		// eoe just used to simplify the client code which can be consistent with non-argo mode.
		if isPodFailed(pod) {
			e = fmt.Errorf("%s Failed, Log: %s\n%s", logPrefix, logURL, r.ErrorMessage())
			logger.Errorf("workflowFailed, %v, spent:%d", e, time.Now().Second()-wf.CreationTimestamp.Second())
			return r.ResponseWithStepComplete(req.Job.Id, "", newStepPhase, eof), e
		}
		logger.Infof("workflowSucceed, spent:%d", time.Now().Second()-wf.CreationTimestamp.Second())

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
		return r.ResponseWithStepComplete(req.Job.Id, stepGroupName, newStepPhase, eof), nil
	}
	return r.Response(req.Job.Id, stepGroupName, newStepPhase, eof), nil
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
