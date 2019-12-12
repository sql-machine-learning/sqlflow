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
	CreateTmpTable   bool
	Select           string
	SQLFlowSubmitter string
}
type coulerFiller struct {
	DataSource    string
	SQLStatements []*sqlStatement
}

const coulerTemplateText = `
import couler.argo as couler
import uuid
datasource = "{{ .DataSource }}"
{{ range $ss := .SQLStatements }}
	{{if $ss.IsExtendedSQL }}
train_sql = '''{{ $ss.OriginalSQL }}'''
# FIXME(typhoonzero): MaxCompute do not support "create table as (select..)" use "create table as select ..." for now.
		{{if $ss.CreateTmpTable }}
tmp_table_name = "_".join(["tmp", uuid.uuid4().hex[:6].upper()])
create_sql = '''CREATE TABLE %s AS %s''' % (tmp_table_name, '''{{$ss.Select}}'''.strip("\n"))
# form a train SQL using the created table
train_sql = train_sql.replace('''{{$ss.Select}}''', "SELECT * FROM %s " % tmp_table_name)
couler.run_container(command='''repl -e "%s" --datasource="%s" && repl -e "%s" --datasource="%s"''' % (create_sql, datasource, train_sql, datasource), image="{{ $ss.DockerImage }}", env={"SQLFLOW_submitter": "{{$ss.SQLFlowSubmitter}}"})
		{{else}}
couler.run_container(command='''repl -e "%s" --datasource="%s"''' % (train_sql, datasource), image="{{ $ss.DockerImage }}", env={"SQLFLOW_submitter": "{{$ss.SQLFlowSubmitter}}"})
		{{end}}
	{{else}}
# TODO(yancey1989): 
#	using "repl -parse" to output IR and
#	feed to "sqlflow_submitter.{submitter}.train" to submite the job
couler.run_container(command='''repl -e "{{ $ss.OriginalSQL }}" --datasource="%s"''' % datasource, image="{{ $ss.DockerImage }}", env={"SQLFLOW_submitter": "{{$ss.SQLFlowSubmitter}}"})
	{{end}}
{{end}}
`

var coulerTemplate = template.Must(template.New("Couler").Parse(coulerTemplateText))
