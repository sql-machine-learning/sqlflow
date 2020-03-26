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

import "text/template"

type sqlStatement struct {
	OriginalSQL   string
	IsExtendedSQL bool
	DockerImage   string
	// CreateTmpTable and Select are used to create a step to generate temporary table for training
	CreateTmpTable bool
	Select         string
	Model          string
	Parameters     string
	IsKatibTrain   bool
}
type coulerFiller struct {
	DataSource       string
	SQLStatements    []*sqlStatement
	SQLFlowSubmitter string
	SQLFlowOSSDir    string
	StepEnvs         map[string]string
	WorkflowTTL      int
	SecretName       string
	SecretData       string
}

const coulerTemplateText = `
import couler.argo as couler
import json
import re
datasource = "{{ .DataSource }}"

step_envs = dict()
{{range $k, $v := .StepEnvs}}
step_envs["{{$k}}"] = '''{{$v}}'''
{{end}}

sqlflow_secret = None
if "{{.SecretName}}" != "":
	# note(yancey1989): set dry_run to true, just reference the secret meta to generate workflow YAML,
	# we should create the secret before launching sqlflowserver
	secret_data=json.loads('''{{.SecretData}}''')
	sqlflow_secret = couler.secret(secret_data, name="{{ .SecretName }}", dry_run=True)

couler.clean_workflow_after_seconds_finished({{.WorkflowTTL}})
def escape_sql(original_sql):
    return re.sub(r'(["$` + "`" + `\\])', r'\\\1', original_sql)

{{ range $ss := .SQLStatements }}
	{{if $ss.IsExtendedSQL }}
train_sql = '''{{ $ss.OriginalSQL }}'''
couler.run_container(command='''repl -e "%s"''' % escape_sql(train_sql), image="{{ $ss.DockerImage }}", env=step_envs, secret=sqlflow_secret)
	{{else if $ss.IsKatibTrain}}
import couler.sqlflow.katib as auto

model = "{{ $ss.Model }}"
params = json.loads('''{{ $ss.Parameters }}''')
train_sql = '''{{ $ss.OriginalSQL }}'''
auto.train(model=model, params=params, sql=escape_sql(train_sql), datasource=datasource)
	{{else}}
# TODO(yancey1989): 
#	using "repl -parse" to output IR and
#	feed to "sqlflow_submitter.{submitter}.train" to submit the job
train_sql = '''{{ $ss.OriginalSQL }}'''
couler.run_container(command='''repl -e "%s"''' % escape_sql(train_sql), image="{{ $ss.DockerImage }}", env=step_envs)
	{{end}}
{{end}}
`

var coulerTemplate = template.Must(template.New("Couler").Parse(coulerTemplateText))
