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

package workflow

import (
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestPatchYAML(t *testing.T) {
	raw := `apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: sqlflow-
spec:
  entrypoint: sqlflow
  templates:
  - name: sqlflow
    steps:
    - - name: sqlflow-76-76
        template: sqlflow-76
  - name: sqlflow-76
    container:
    image: some-image
    command:
      - bash
      - -c
      - step -e "show databases;"
  ttlSecondsAfterFinished: 600
`
	obj := make(map[interface{}]interface{})
	e := yaml.Unmarshal([]byte(raw), &obj)
	if e != nil {
		t.Fatalf("%v", e)
	}
	fmt.Printf("obj: %v\n", obj)

	spec, ok := obj["spec"].(map[interface{}]interface{})
	if !ok {
		t.Fatalf("parse sepc error")
	}
	spec["serviceAccountName"] = "someSA"
	yamlBytes, e := yaml.Marshal(obj)
	if e != nil {
		t.Fatalf("%v", e)
	}
	result := string(yamlBytes)
	fmt.Printf("result: %s", result)
	if !strings.Contains(result, "serviceAccountName: someSA") {
		t.Fatalf("yaml string do not patched")
	}
}
