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
	"os/exec"
	"regexp"
	"strings"
)

// Tekton submites TaskRun and fetches the step status
type Tekton struct{}

// Submit the Tekton YAML and returns the TaskRun name
func (p *Tekton) Submit(yaml string) (string, error) {
	cmd := exec.Command("kubectl", "create", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("submit tekton YAML error: %v, output: %s", err, string(output))
	}
	re := regexp.MustCompile(`taskrun.tekton.dev/(.+) .+`)
	createRes := re.FindStringSubmatch(string(output))
	if len(createRes) != 2 {
		return "", fmt.Errorf("parse created resource error: %s, %v", cmd, output)
	}
	return createRes[1], nil
}
