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
	Into           string
}

// Filler is used to fill the template
type Filler struct {
	DataSource       string
	SQLStatements    []*sqlStatement
	SQLFlowSubmitter string
	SQLFlowOSSDir    string
	StepEnvs         map[string]string
	WorkflowTTL      int
	SecretName       string
	SecretData       string
	Resources        string
	ClusterConfigFn  string
	// The step output duplication file, in the container.
	// Log framework may read the file to collect logs.
	StepLogFile        string
	UseCoulerSubmitter bool
	CoulerServerAddr   string
	CoulerCluster      string
}

const coulerTemplateText = `
import json
import re
import os
import couler.argo as couler
datasource = "{{ .DataSource }}"
step_log_file = None
if "{{ .StepLogFile }}" != "":
  step_log_file = "{{ .StepLogFile }}"
cluster_config = None
if "{{.ClusterConfigFn}}" != "":
  cluster_config = "{{.ClusterConfigFn}}"
workflow_ttl = {{.WorkflowTTL}}

# it's bug of the couler project, that needs "" on integer environment variable value to avoid the 
# workflow failed: "invalid spec: cannot convert int64 to string"
# issue: https://github.com/couler-proj/couler/issues/108
def escape_env(value):
	return str(value).replace('"', '\\"')

step_envs = dict()
{{range $k, $v := .StepEnvs}}
step_envs["{{$k}}"] = '''"%s"''' % escape_env('''{{$v}}''')
{{end}}

def step_command(sql, step_log_file):
	if step_log_file:
		# wait for some seconds to exit in case the
		# step pod is recycled too fast
		exit_time_wait = os.getenv("SQLFLOW_WORKFLOW_EXIT_TIME_WAIT", "0")

		log_dir = os.path.dirname(step_log_file)
		return "".join([
			"if [[ -f /opt/sqlflow/init_step_container.sh ]]; "
			"then bash /opt/sqlflow/init_step_container.sh; fi",
			" && set -o pipefail",  # fail when any sub-command fail
			" && mkdir -p %s" % log_dir,
			""" && (step -e "%s" 2>&1 | tee %s)""" %
			(sql, step_log_file),
			" && sleep %s" % exit_time_wait
		])
	else:
		return '''step -e "%s"''' % sql

sqlflow_secret = None
if "{{.SecretName}}" != "":
	# note(yancey1989): set dry_run to true, just reference the secret meta to generate workflow YAML,
	# we should create the secret before launching sqlflowserver
	secret_data=json.loads('''{{.SecretData}}''')
	sqlflow_secret = couler.create_secret(secret_data, name="{{ .SecretName }}", dry_run=True)

resources = None
if '''{{.Resources}}''' != "":
  resources=json.loads('''{{.Resources}}''')

{{ range $ss := .SQLStatements }}
	{{if $ss.IsExtendedSQL }}
couler.run_container(command=["bash", "-c", step_command('''{{ $ss.OriginalSQL}}''', step_log_file)],
  image="{{ $ss.DockerImage}}",
  env=step_envs,
  secret=sqlflow_secret,
  resources=resources)
  {{else if $ss.IsKatibTrain}}
import couler.sqlflow.katib as auto

model = "{{ $ss.Model }}"
params = json.loads('''{{ $ss.Parameters }}''')
train_sql = '''{{ $ss.OriginalSQL }}'''
auto.train(model=model, params=params, sql=escape_sql(train_sql), datasource=datasource)
	{{else}}
couler.run_container(command=["bash", "-c", step_command('''{{ $ss.OriginalSQL}}''', step_log_file)],
	image="{{ $ss.DockerImage}}",
	env=step_envs,
	secret=sqlflow_secret,
	resources=resources)
	{{end}}
{{end}}

couler.config_workflow(cluster_config_file=cluster_config, time_to_clean=workflow_ttl)
{{ if .UseCoulerSubmitter }}
from ant_couler.couler_submitter import CoulerSubmitter
sbmtr = CoulerSubmitter(address="{{.CoulerServerAddr}}", namespace="kubemaker")
resp = couler.run(submitter=sbmtr, cluster="{{.CoulerCluster}}")
print(resp)
{{ endif }}
`

var coulerTemplate = template.Must(template.New("Couler").Parse(coulerTemplateText))
