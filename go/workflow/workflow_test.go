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
	yaml_bytes, e := yaml.Marshal(obj)
	if e != nil {
		t.Fatalf("%v", e)
	}
	result := string(yaml_bytes)
	fmt.Printf("result: %s", result)
	if !strings.Contains(result, "serviceAccountName: someSA") {
		t.Fatalf("yaml string do not patched")
	}
}
