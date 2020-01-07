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
}

const coulerTemplateText = `
import couler.argo as couler
import uuid
datasource = "{{ .DataSource }}"
envs = {"SQLFLOW_submitter": "{{.SQLFlowSubmitter}}",
        "SQLFLOW_OSS_CHECKPOINT_DIR": "{{.SQLFlowOSSDir}}"}
{{ range $ss := .SQLStatements }}
	{{if $ss.IsExtendedSQL }}
train_sql = '''{{ $ss.OriginalSQL }}'''
couler.run_container(command='''repl -e "%s" --datasource="%s"''' % (train_sql, datasource), image="{{ $ss.DockerImage }}", env=envs)
	{{else if $ss.IsKatibTrain}}
import couler.sqlflow.katib as auto

model = "{{ $ss.Model }}"
params = json.loads('''{{ $ss.Parameters }}''')
train_sql = '''{{ $ss.OriginalSQL }}'''
auto.train(model=model, params=params, sql=train_sql, datasource=datasource)
	{{else}}
# TODO(yancey1989): 
#	using "repl -parse" to output IR and
#	feed to "sqlflow_submitter.{submitter}.train" to submit the job
couler.run_container(command='''repl -e "{{ $ss.OriginalSQL }}" --datasource="%s"''' % datasource, image="{{ $ss.DockerImage }}", env=envs)
	{{end}}
{{end}}
`

var coulerTemplate = template.Must(template.New("Couler").Parse(coulerTemplateText))
