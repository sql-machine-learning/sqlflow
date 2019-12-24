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

type coulerKatibFiller struct {
	OriginalSQL   string
	IsExtendedSQL bool
	DockerImage   string
	// CreateTmpTable and Select are used to create a step to generate temporary table for training
	CreateTmpTable  bool
	Select          string
	Validation      string
	ModelParamsJSON string
	Model           string
	FieldDescJSON   string
	LabelJSON       string
}

const coulerKatibTemplateText = `
import couler.argo as couler
import couler.katib.sqlflow as auto
import uuid

model = "{{ .Model }}"
params = json.loads('''{{ .ModelParamsJSON }}''')

data = {}
data["select"] = "{{ .Select }}"
data["validation"] = "{{ .Validation }}"
data["features"] = json.loads('''{{ .FieldDescJSON }}''')
data["labels"] = json.loads('''{{ .LabelJSON }}''')

auto.train( model=model, hyperparameters=params, data=data )

`

var coulerKatibTemplate = template.Must(template.New("CoulerKatib").Parse(coulerKatibTemplateText))
