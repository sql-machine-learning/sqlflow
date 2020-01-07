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

package couler

import (
	"io/ioutil"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

func TestCodegen(t *testing.T) {
	a := assert.New(t)
	sqlIR := mockSQLProgramIR()
	code, err := Run(sqlIR, &pb.Session{})
	a.NoError(err)

	r, _ := regexp.Compile(`repl -e "(.*);"`)
	a.Equal(r.FindStringSubmatch(code)[1], "SELECT * FROM iris.train limit 10")
}
func mockSQLProgramIR() ir.SQLProgram {
	standardSQL := ir.StandardSQL("SELECT * FROM iris.train limit 10;")
	trainStmt := ir.MockTrainStmt(false)
	return []ir.SQLStatement{&standardSQL, trainStmt}
}

var testCoulerClusterConfig = `
class K8s(object):
    def __init__(self):
        pass

    def with_pod(self, template):
        self._with_tolerations(template)
        return template

    def _with_tolerations(self, template):
        template["tolerations"] = list()
        template["tolerations"].append({
          "effect": "NoSchedule",
          "key": "key",
          "value": "value",
          "operator": "Equal"
        })

cluster = K8s()
`

var expectedArgoYAML = `apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: test-sqlflow-couler-
spec:
  entrypoint: test-sqlflow-couler
  templates:
    - name: test-sqlflow-couler
      steps:
        - - name: test-sqlflow-couler-3-3
            template: test-sqlflow-couler-3
    - name: test-sqlflow-couler-3
      container:
        image: docker/whalesay
        command:
          - bash
          - -c
          - 'echo "SQLFlow bridges AI and SQL engine."'
      tolerations:
        - effect: NoSchedule
          key: key
          operator: Equal
          value: value

`

var testCoulerProgram = `
import couler.argo as couler
couler.run_container(image="docker/whalesay", command='echo "SQLFlow bridges AI and SQL engine."')
`

func TestWriteArgoYamlWithClusterConfig(t *testing.T) {
	a := assert.New(t)

	coulerFileName := "/tmp/test-sqlflow-couler.py"
	e := ioutil.WriteFile(coulerFileName, []byte(testCoulerProgram), 0755)
	a.NoError(e)
	defer os.Remove(coulerFileName)

	cfFileName := "/tmp/sqlflow-cluster.py"
	e = ioutil.WriteFile(cfFileName, []byte(testCoulerClusterConfig), 0755)
	a.NoError(e)
	defer os.Remove(cfFileName)

	os.Setenv("SQLFLOW_COULER_CLUSTER_CONFIG", cfFileName)

	argoFile, e := writeArgoFile(coulerFileName)
	a.NoError(e)
	defer os.Remove(argoFile)

	out, e := ioutil.ReadFile(argoFile)
	a.NoError(e)

	a.Equal(string(out), expectedArgoYAML)
}
