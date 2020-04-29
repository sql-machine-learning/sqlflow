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
	OSSModelDir        string
	DataSource         string
	TrainSelect        string
	ValidationSelect   string
	ModelParamsJSON    string
	TrainParamsJSON    string
	FieldDescJSON      string
	FeatureColumnNames []string
	LabelJSON          string
	DiskCache          bool
	BatchSize          int
	Epoch              int
	IsPAI              bool
	PAITrainTable      string
	PAIValidateTable   string
	Workers            int
}

const trainTemplateText = `
from sqlflow_submitter.xgboost.train import train
from sqlflow_submitter.tensorflow.pai_distributed import define_tf_flags, set_oss_environs
import json

if "{{.IsPAI}}" == "true":
    FLAGS = define_tf_flags()
    set_oss_environs(FLAGS)

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
      disk_cache="{{.DiskCache}}" == "true",
      batch_size={{.BatchSize}},
      epoch={{.Epoch}},
      is_pai="{{.IsPAI}}" == "true",
      pai_train_table="{{.PAITrainTable}}",
      pai_validate_table="{{.PAIValidateTable}}",
      oss_model_dir="{{.OSSModelDir}}")
`

const distTrainTemplateText = `
from sqlflow_submitter.xgboost.train import dist_train
from sqlflow_submitter.tensorflow.pai_distributed import define_tf_flags, set_oss_environs
import json

FLAGS = define_tf_flags()
set_oss_environs(FLAGS)

model_params = json.loads('''{{.ModelParamsJSON}}''')
train_params = json.loads('''{{.TrainParamsJSON}}''')
feature_metas = json.loads('''{{.FieldDescJSON}}''')
label_meta = json.loads('''{{.LabelJSON}}''')

feature_column_names = [{{range .FeatureColumnNames}}
"{{.}}",
{{end}}]

dist_train(flags=FLAGS,
      num_workers={{.Workers}},
      datasource='''{{.DataSource}}''',
      select='''{{.TrainSelect}}''',
      model_params=model_params,
      train_params=train_params,
      feature_metas=feature_metas,
      feature_column_names=feature_column_names,
      label_meta=label_meta,
      validation_select='''{{.ValidationSelect}}''',
      disk_cache="{{.DiskCache}}" == "true",
      batch_size={{.BatchSize}},
      epoch={{.Epoch}},
      is_pai="{{.IsPAI}}" == "true",
      pai_train_table="{{.PAITrainTable}}",
      pai_validate_table="{{.PAIValidateTable}}",
      oss_model_dir="{{.OSSModelDir}}")
`

var trainTemplate = template.Must(template.New("Train").Parse(trainTemplateText))
var distTrainTemplate = template.Must(template.New("DistTrain").Parse(distTrainTemplateText))
