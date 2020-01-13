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
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/pkg/database"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

func TestCodegen(t *testing.T) {
	a := assert.New(t)
	sqlIR := mockSQLProgramIR()
	os.Setenv("SQLFLOW_ALISA_OSS_AK", "oss_key")
	defer os.Unsetenv("SQLFLOW_ALISA_OSS_AK")
	code, err := Run(sqlIR, &pb.Session{})
	a.NoError(err)

	r, _ := regexp.Compile(`repl -e "(.*);"`)
	a.Equal(r.FindStringSubmatch(code)[1], "SELECT * FROM iris.train limit 10")

	a.True(strings.Contains(code, `step_envs["SQLFLOW_ALISA_OSS_AK"] = "oss_key"`))
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

func TestKatibCodegen(t *testing.T) {
	a := assert.New(t)
	os.Setenv("SQLFLOW_submitter", "katib")

	cfg := database.GetTestingMySQLConfig()

	standardSQL := ir.StandardSQL("SELECT * FROM iris.train limit 10;")
	sqlIR := MockKatibTrainStmt(fmt.Sprintf("mysql://%s", cfg.FormatDSN()))

	program := []ir.SQLStatement{&standardSQL, &sqlIR}

	_, err := Run(program, &pb.Session{})

	a.NoError(err)
}

func MockKatibTrainStmt(datasource string) ir.TrainStmt {
	attrs := map[string]interface{}{}

	attrs["objective"] = "multi:softprob"
	attrs["eta"] = float32(0.1)
	attrs["range.max_depth"] = []int{2, 10}
	estimator := "xgboost.gbtree"

	return ir.TrainStmt{
		OriginalSQL: `
SELECT *
FROM iris.train
TO TRAIN xgboost.gbtree
WITH
	objective = "multi:softprob"
	eta = 0.1
	max_depth = [2, 10]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_xgboost_model;
`,
		Select:           "select * from iris.train;",
		ValidationSelect: "select * from iris.test;",
		Estimator:        estimator,
		Attributes:       attrs,
		Features: map[string][]ir.FeatureColumn{
			"feature_columns": {
				&ir.NumericColumn{&ir.FieldDesc{"sepal_length", ir.Float, "", []int{1}, false, nil, 0}},
				&ir.NumericColumn{&ir.FieldDesc{"sepal_width", ir.Float, "", []int{1}, false, nil, 0}},
				&ir.NumericColumn{&ir.FieldDesc{"petal_length", ir.Float, "", []int{1}, false, nil, 0}},
				&ir.NumericColumn{&ir.FieldDesc{"petal_width", ir.Float, "", []int{1}, false, nil, 0}}}},
		Label: &ir.NumericColumn{&ir.FieldDesc{"class", ir.Int, "", []int{1}, false, nil, 0}}}
}
