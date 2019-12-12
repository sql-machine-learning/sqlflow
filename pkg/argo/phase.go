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
	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

func translatePhase(nodePhase wfv1.NodePhase) pb.FetchResponse_Phase {
	switch nodePhase {
	case wfv1.NodePending:
		return pb.FetchResponse_PENDING
	case wfv1.NodeRunning:
		return pb.FetchResponse_RUNNING
	case wfv1.NodeSucceeded:
		return pb.FetchResponse_SUCCEEDED
	case wfv1.NodeSkipped:
		return pb.FetchResponse_SKIPPED
	case wfv1.NodeFailed:
		return pb.FetchResponse_FAILED
	case wfv1.NodeError:
		return pb.FetchResponse_ERROR
	default:
		panic(fmt.Sprintf("unrecognized node phase %v", nodePhase))
	}
}

func isCompletedPhaseWF(phase wfv1.NodePhase) bool {
	return phase == wfv1.NodeSucceeded ||
		phase == wfv1.NodeFailed ||
		phase == wfv1.NodeError ||
		phase == wfv1.NodeSkipped
}

func isCompletePhasePB(phase pb.FetchResponse_Phase) bool {
	return phase == pb.FetchResponse_SUCCEEDED ||
		phase == pb.FetchResponse_SKIPPED ||
		phase == pb.FetchResponse_FAILED ||
		phase == pb.FetchResponse_ERROR
}
