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

package main

import "text/template"

type trainFiller struct {
	DataSource       string
	TrainSelect      string
	ValidationSelect string
	ModelParamsJSON  string
	FieldMetaJSON    string
}

const trainTemplateText = `
import json
from KatibXGboostExperiment import KatibXGBoostExperiment


model_params = json.loads('''{{.ModelParamsJSON}}''')
feature_names = json.loads('''{{.FieldMetaJSON}}''')

select = '''{{.TrainSelect}}'''
validate_select = '''{{.ValidationSelect}}'''

exp_name = "test1"

katib_xgboost_experiment = KatibXGBoostExperiment(select, validate_select, feature_names, exp_name, model_params )
katib_xgboost_experiment.submit_xgboost_experiment()


`

var trainTemplate = template.Must(template.New("Train").Parse(trainTemplateText))
