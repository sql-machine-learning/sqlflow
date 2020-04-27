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

package fluid

import "text/template"

const fluidTemplateText = `
import fluid

step_envs = dict()
{{range $k, $v := .StepEnvs}}
step_envs["{{$k}}"] = '''{{$v}}'''
{{end}}

@fluid.task
def sqlflow_workflow():
{{ range $ss := .SQLStatements }}
    fluid.step(image="{{ $ss.DockerImage }}", cmd=["step"], args=["-e", '''{{ $ss.OriginalSQL }}'''], env=step_envs)
{{ end }}

sqlflow_workflow()
`

var fluidTemplate = template.Must(template.New("Fluid").Parse(fluidTemplateText))
