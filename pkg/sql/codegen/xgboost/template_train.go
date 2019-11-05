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

package xgboost

import "text/template"

type trainFiller struct {
	DataSource       string
	TrainSelect      string
	ValidationSelect string
	ModelParamsJSON  string
	TrainParamsJSON  string
	FieldMetaJSON    string
	LabelJSON        string
}

const trainTemplateText = `
import xgboost as xgb
from sqlflow_submitter.xgboost.train import train
from sqlflow_submitter.db import connect_with_data_source, db_generator
import json
model_params = json.loads('''{{.ModelParamsJSON}}''')
train_params = json.loads('''{{.TrainParamsJSON}}''')
feature_field_meta = json.loads('''{{.FieldMetaJSON}}''')
label_field_meta = json.loads('''{{.LabelJSON}}''')

train(datasource='''{{.DataSource}}''',
      select='''{{.TrainSelect}}''',
      model_params=model_params,
      train_params=train_params,
      feature_field_meta=feature_field_meta,
      label_field_meta=label_field_meta,
      validation_select='''{{.ValidationSelect}}''')
`

var trainTemplate = template.Must(template.New("Train").Parse(trainTemplateText))
