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
	"time"

	"sqlflow.org/sqlflow/pkg/log"
	pb "sqlflow.org/sqlflow/pkg/proto"
	wfrsp "sqlflow.org/sqlflow/pkg/workflow/response"
)

// Fetch fetches the Tekton Step status
func (t *Tekton) Fetch(req *pb.FetchRequest) (*pb.FetchResponse, error) {
	logger := log.WithFields(log.Fields{
		"requestID": log.UUID(),
		"jobID":     req.Job.Id,
		"stepID":    req.StepId,
		"event":     "fetch",
	})
	w, e := newWorkflow(req.Job.Id, req.StepId)
	if e != nil {
		logger.Errorf("setup workflow error: %v", e)
		return nil, e
	}

	if w.isPending() {
		return wfrsp.New(0, 0).Response(req.Job.Id, "", "", false), nil
	}

	stepCnt, stepIdx, e := w.stepIdx()
	if e != nil {
		return nil, e
	}

	r := wfrsp.New(stepCnt, stepIdx)
	updatedStepPhase := req.StepPhase
	updatedStepID := w.stepID
	eof := false

	if req.StepPhase == "" {
		// return step log url for the first Fetch request of this step
		execCode, e := w.stepCode()
		if e != nil {
			return nil, e
		}
		r.AppendMessage(fmt.Sprintf("Execute Code: %s", execCode))
		r.AppendMessage(fmt.Sprintf("Log: %s", w.stepLogURL()))
	}

	if w.stepPhase() != req.StepPhase {
		// append the updated step phase to the workflow response
		r.AppendMessage(fmt.Sprintf("Status: %s", w.stepPhase()))
		updatedStepPhase = w.stepPhase()
	}

	if w.isStepComplete() {
		if w.isStepFailed() {
			logger.Errorf("workflowFailed, spent:%d", time.Now().Second()-w.creationSecond())
			return r.ResponseWithEOE(req.Job.Id, "", updatedStepPhase, eof),
				fmt.Errorf("SQLFlow Step [%d/%d] Failed, Log: %s", stepIdx, stepCnt, w.stepLogURL())
		}
		logger.Infof("workflowSucceed, spent:%d", time.Now().Second()-w.creationSecond())

		stepLogs, e := w.stepLogs()
		if e != nil {
			return nil, e
		}
		if e := r.AppendProtoMessages(stepLogs); e != nil {
			return nil, e
		}
		if w.nextStepID() == "" {
			eof = true
		} else {
			updatedStepPhase = ""
			updatedStepID = w.nextStepID()
		}
		return r.ResponseWithEOE(req.Job.Id, updatedStepID, updatedStepPhase, eof), nil

	}
	return r.Response(req.Job.Id, updatedStepID, updatedStepPhase, eof), nil
}
