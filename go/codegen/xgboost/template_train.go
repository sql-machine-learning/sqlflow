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
	OSSModelDirToSave   string
	OSSModelDirToLoad   string
	DataSource          string
	TrainSelect         string
	ValidationSelect    string
	ModelParamsJSON     string
	TrainParamsJSON     string
	FieldDescJSON       string
	FeatureColumnNames  []string
	LabelJSON           string
	FeatureColumnCode   string
	DiskCache           bool
	BatchSize           int
	Epoch               int
	LoadPreTrainedModel bool
	IsPAI               bool
	PAITrainTable       string
	PAIValidateTable    string
	ModelRepoImage      string
	OriginalSQL         string
}

const trainTemplateText = `
import runtime.xgboost as xgboost_extended
from runtime.xgboost.train import train
import runtime.xgboost.feature_column as xgboost_feature_column
from runtime.pai.pai_distributed import define_tf_flags, set_oss_environs
import json

if "{{.IsPAI}}" == "true":
    FLAGS = define_tf_flags()
    set_oss_environs(FLAGS)

if "{{.IsPAI}}" == "true" and "{{.LoadPreTrainedModel}}" == "true":
    from runtime.model import oss
    oss.load_file("{{.OSSModelDirToLoad}}", "my_model")

model_params = json.loads('''{{.ModelParamsJSON}}''')
train_params = json.loads('''{{.TrainParamsJSON}}''')
feature_metas = json.loads('''{{.FieldDescJSON}}''')
label_meta = json.loads('''{{.LabelJSON}}''')

feature_column_names = [{{range .FeatureColumnNames}}
"{{.}}",
{{end}}]

# NOTE: in the current implementation, we are generating a transform_fn from COLUMN clause. 
# The transform_fn is executed during the process of dumping the original data into DMatrix SVM file.
feature_column_list = [{{.FeatureColumnCode}}]
transform_fn = xgboost_extended.feature_column.ComposedColumnTransformer(feature_column_names, *feature_column_list)

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
      load_pretrained_model="{{.LoadPreTrainedModel}}" == "true",
      is_pai="{{.IsPAI}}" == "true",
      pai_train_table="{{.PAITrainTable}}",
      pai_validate_table="{{.PAIValidateTable}}",
      oss_model_dir="{{.OSSModelDirToSave}}",
      transform_fn=transform_fn,
      feature_column_code='''{{.FeatureColumnCode}}''',
      model_repo_image="{{.ModelRepoImage}}",
      original_sql='''{{.OriginalSQL}}''')
`

const distTrainTemplateText = `
import runtime.xgboost as xgboost_extended
from runtime.xgboost.train import dist_train
from runtime.pai.pai_distributed import define_tf_flags, set_oss_environs
import json

FLAGS = define_tf_flags()
set_oss_environs(FLAGS)

if "{{.IsPAI}}" == "true" and "{{.LoadPreTrainedModel}}" == "true":
	from runtime.model import oss
	oss.load_file("{{.OSSModelDirToLoad}}", "my_model")

model_params = json.loads('''{{.ModelParamsJSON}}''')
train_params = json.loads('''{{.TrainParamsJSON}}''')
feature_metas = json.loads('''{{.FieldDescJSON}}''')
label_meta = json.loads('''{{.LabelJSON}}''')

feature_column_names = [{{range .FeatureColumnNames}}
"{{.}}",
{{end}}]

# NOTE: in the current implementation, we are generating a transform_fn from COLUMN clause. 
# The transform_fn is executed during the process of dumping the original data into DMatrix SVM file.
transform_fn = xgboost_extended.feature_column.ComposedColumnTransformer(feature_column_names, {{.FeatureColumnCode}})

dist_train(flags=FLAGS,
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
      load_pretrained_model="{{.LoadPreTrainedModel}}" == "true",
      is_pai="{{.IsPAI}}" == "true",
      pai_train_table="{{.PAITrainTable}}",
      pai_validate_table="{{.PAIValidateTable}}",
      oss_model_dir="{{.OSSModelDirToSave}}",
      transform_fn=transform_fn,
      feature_column_code='''{{.FeatureColumnCode}}''',
      model_repo_image="{{.ModelRepoImage}}",
      original_sql='''{{.OriginalSQL}}''')
`

var trainTemplate = template.Must(template.New("Train").Parse(trainTemplateText))
var distTrainTemplate = template.Must(template.New("DistTrain").Parse(distTrainTemplateText))
