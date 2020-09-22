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

package couler

import (
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

var testCoulerClusterConfig = `
class K8s(object):
    def __init__(self):
        pass

    def with_pod(self, template):
        self._with_tolerations(template)
        return template

    def with_workflow_spec(self, spec):
        spec["hostNetwork"] = True
        return spec

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
  generateName: sqlflow-
spec:
  entrypoint: sqlflow
  hostNetwork: true
  templates:
    - name: sqlflow
      steps:
        - - name: sqlflow-3-3
            template: sqlflow-3
    - name: sqlflow-3
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

func TestCoulerCodegen(t *testing.T) {
	a := assert.New(t)
	sqlIR := MockSQLProgramIR()
	os.Setenv("SQLFLOW_OSS_AK", "oss_key")
	os.Setenv("SQLFLOW_WORKFLOW_SECRET", `{"sqlflow-secret":{"oss_sk": "oss_sk"}}`)
	os.Setenv(envResource, `{"memory": "32Mi", "cpu": "100m"}`)
	defer os.Unsetenv("SQLFLOW_OSS_AK")
	cg := &Codegen{}
	code, err := cg.GenCode(sqlIR, &pb.Session{})
	a.NoError(err)

	r, e := regexp.Compile(`steps.sqlflow\(sql=r'''(.*);''', `)
	a.NoError(e)
	a.Equal(r.FindStringSubmatch(code)[1], "SELECT * FROM iris.train limit 10")
	a.True(strings.Contains(code, `step_envs["SQLFLOW_OSS_AK"] = '''oss_key'''`))
	a.False(strings.Contains(code, `step_envs["SQLFLOW_WORKFLOW_SECRET"]`))
	a.True(strings.Contains(code, `couler.clean_workflow_after_seconds_finished(86400)`))
	a.True(strings.Contains(code, `couler.secret(secret_data, name="sqlflow-secret", dry_run=True)`))
	a.True(strings.Contains(code, `resources=json.loads('''{"memory": "32Mi", "cpu": "100m"}''')`))

	_, e = cg.GenYAML(code)
	yaml, e := cg.GenYAML(code)
	a.NoError(e)
	r, e = regexp.Compile(`step -e "(.*);"`)
	a.NoError(e)
	a.Equal("SELECT * FROM iris.train limit 10", r.FindStringSubmatch(yaml)[1])
	a.NoError(e)

	os.Setenv("SQLFLOW_WORKFLOW_STEP_LOG_FILE", "/home/admin/logs/step.log")
	defer os.Unsetenv("SQLFLOW_WORKFLOW_STEP_LOG_FILE")
	code, err = cg.GenCode(sqlIR, &pb.Session{})
	a.NoError(err)
	r, e = regexp.Compile(`step_log_file = "(.*)"`)
	a.True(strings.Contains(code, `step_log_file = "/home/admin/logs/step.log"`))
	a.True(strings.Contains(code, "log_file=step_log_file"))

	yaml, e = cg.GenYAML(code)
	a.NoError(e)
	r, e = regexp.Compile(`mkdir -p (.*) && \(step -e "([^|]|\n)*[|] tee (.*)\)`)
	a.NoError(e)
	a.Equal("/home/admin/logs", r.FindStringSubmatch(yaml)[1])
	a.Equal("/home/admin/logs/step.log", r.FindStringSubmatch(yaml)[3])
	a.NoError(e)

	r, e = regexp.Compile("- name: SQLFLOW_WORKFLOW_STEP_LOG_FILE\n.*value: '(.*)'")
	a.NoError(e)
	a.Equal("/home/admin/logs/step.log", r.FindStringSubmatch(yaml)[1])
}

func TestCoulerCodegenSpecialChars(t *testing.T) {
	a := assert.New(t)
	specialCharsStmt := ir.NormalStmt("`$\"\\;")
	sqlIR := []ir.SQLFlowStmt{&specialCharsStmt}
	cg := &Codegen{}
	code, err := cg.GenCode(sqlIR, &pb.Session{})
	a.NoError(err)
	yaml, e := cg.GenYAML(code)
	a.NoError(e)
	r, _ := regexp.Compile(`step -e "(.*);"`)
	a.Equal("\\`\\$\\\"\\\\", r.FindStringSubmatch(yaml)[1])
}

func TestStringInStringSQL(t *testing.T) {
	a := assert.New(t)
	specialCharsStmt := ir.TrainStmt{
		OriginalSQL: `
		SELECT * FROM iris.train TO TRAIN	DNNClassifier
		WITH n_classes=3
		validation.select="select * from iris.train where name like \"Versicolor\";"
		LABEL=class
		INTO my_iris_model`,
	}
	sqlIR := []ir.SQLFlowStmt{&specialCharsStmt}
	cg := &Codegen{}
	code, err := cg.GenCode(sqlIR, &pb.Session{})
	a.NoError(err)
	yaml, e := cg.GenYAML(code)
	a.NoError(e)
	expect := `validation.select=\\\"select * from iris.train where\
            \ name like \\\\\\\"Versicolor\\\\\\\";\\\"`
	a.True(strings.Contains(yaml, expect))
}

func mockSQLProgramIR() []ir.SQLFlowStmt {
	standardSQL := ir.NormalStmt("SELECT * FROM iris.train limit 10;")
	trainStmt := ir.MockTrainStmt(false)
	return []ir.SQLFlowStmt{&standardSQL, trainStmt}
}

func TestCompileCoulerProgram(t *testing.T) {
	a := assert.New(t)

	cfFileName := "/tmp/sqlflow-cluster.py"
	e := ioutil.WriteFile(cfFileName, []byte(testCoulerClusterConfig), 0755)
	a.NoError(e)
	defer os.Remove(cfFileName)

	os.Setenv("SQLFLOW_WORKFLOW_CLUSTER_CONFIG", cfFileName)
	defer os.Unsetenv("SQLFLOW_WORKFLOW_CLUSTER_CONFIG")
	cg := &Codegen{}
	out, e := cg.GenYAML(testCoulerProgram)
	a.NoError(e)

	a.Equal(expectedArgoYAML, out)
}

func TestKatibCodegen(t *testing.T) {
	a := assert.New(t)
	os.Setenv("SQLFLOW_submitter", "katib")

	standardSQL := ir.NormalStmt("SELECT * FROM iris.train limit 10;")
	sqlIR := MockKatibTrainStmt(database.GetTestingMySQLURL())

	program := []ir.SQLFlowStmt{&standardSQL, &sqlIR}

	cg := &Codegen{}
	_, err := cg.GenCode(program, &pb.Session{})

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
				&ir.NumericColumn{&ir.FieldDesc{"sepal_length", ir.Float, ir.Float, "", "", "", []int{1}, false, nil, 0}},
				&ir.NumericColumn{&ir.FieldDesc{"sepal_width", ir.Float, ir.Float, "", "", "", []int{1}, false, nil, 0}},
				&ir.NumericColumn{&ir.FieldDesc{"petal_length", ir.Float, ir.Float, "", "", "", []int{1}, false, nil, 0}},
				&ir.NumericColumn{&ir.FieldDesc{"petal_width", ir.Float, ir.Float, "", "", "", []int{1}, false, nil, 0}}}},
		Label: &ir.NumericColumn{&ir.FieldDesc{"class", ir.Int, ir.Float, "", "", "", []int{1}, false, nil, 0}}}
}
