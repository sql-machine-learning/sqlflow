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

package xgboost

import "text/template"

type trainFiller struct {
	DataSource         string
	TrainSelect        string
	ValidationSelect   string
	ModelParamsJSON    string
	TrainParamsJSON    string
	FieldDescJSON      string
	FeatureColumnNames []string
	LabelJSON          string
	IsPAI              bool
	PAITrainTable      string
	PAIValidateTable   string
}

const trainTemplateText = `
from sqlflow_submitter.xgboost.train import train
import json

model_params = json.loads('''{{.ModelParamsJSON}}''')
train_params = json.loads('''{{.TrainParamsJSON}}''')
feature_metas = json.loads('''{{.FieldDescJSON}}''')
label_meta = json.loads('''{{.LabelJSON}}''')

feature_column_names = [{{range .FeatureColumnNames}}
"{{.}}",
{{end}}]

train(datasource='''{{.DataSource}}''',
        select='''{{.TrainSelect}}''',
        model_params=model_params,
				train_params=train_params,
        feature_metas=feature_metas,
        feature_column_names=feature_column_names,
        label_meta=label_meta,
        validation_select='''{{.ValidationSelect}}''',
        is_pai="{{.IsPAI}}" == "true",
        pai_train_table="{{.PAITrainTable}}",
        pai_validate_table="{{.PAIValidateTable}}")
`

var trainTemplate = template.Must(template.New("Train").Parse(trainTemplateText))
