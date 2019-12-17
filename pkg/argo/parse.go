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
	"encoding/json"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func parseWorkflowResource(b []byte) (*wfv1.Workflow, error) {
	wf := wfv1.Workflow{}
	return &wf, json.Unmarshal(b, &wf)
}

func parsePodResource(b []byte) (*corev1.Pod, error) {
	pod := corev1.Pod{}
	return &pod, json.Unmarshal(b, &pod)
}
