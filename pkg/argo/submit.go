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
)

// Submit the Argo workflow and returns the workflow ID
func Submit(argoFileName string) (string, error) {
	// submit Argo YAML and fetch the workflow ID.
	cmd := exec.Command("kubectl", "create", "-f", argoFileName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("submit Argo YAML error: %v, output: %s", err, string(output))
	}

	return getWorkflowID(string(output))
}

func getWorkflowID(output string) (string, error) {
	reWorkflow := regexp.MustCompile(`.+/(.+) .+`)
	wf := reWorkflow.FindStringSubmatch(string(output))
	if len(wf) != 2 {
		return "", fmt.Errorf("parse workflow ID error: %v", output)
	}

	return wf[1], nil
}
