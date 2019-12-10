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

package tensorflow

import "sqlflow.org/sqlflow/pkg/sql/ir"

type trainFiller struct {
	DataSource        string
	TrainSelect       string
	ValidationSelect  string
	Estimator         string
	IsKerasModel      bool
	FieldMetas        []*ir.FieldMeta
	FeatureColumnCode string
	Y                 *ir.FieldMeta
	ModelParams       map[string]interface{}
	TrainParams       map[string]interface{}
	ValidationParams  map[string]interface{}
	Save              string
	IsPAI             bool
	PAITrainTable     string
}

const tfTrainTemplateText = `
from sqlflow_submitter.tensorflow.train import train
import tensorflow as tf
try:
    import sqlflow_models
except:
    pass

feature_column_names = [{{range .FieldMetas}}
"{{.Name}}",
{{end}}]

feature_metas = dict()
{{ range $value := .FieldMetas }}
feature_metas["{{$value.Name}}"] = {
    "feature_name": "{{$value.Name}}",
    "dtype": "{{$value.DType | dtypeToString}}",
    "delimiter": "{{$value.Delimiter}}",
    "shape": {{$value.Shape | intArrayToJSONString}},
    "is_sparse": "{{$value.IsSparse}}" == "true"
}
{{end}}

label_meta = {
    "feature_name": "{{.Y.Name}}",
    "dtype": "{{.Y.DType | dtypeToString}}",
    "delimiter": "{{.Y.Delimiter}}",
    "shape": {{.Y.Shape | intArrayToJSONString}},
    "is_sparse": "{{.Y.IsSparse}}" == "true"
}

model_params=dict()
{{range $k, $v := .ModelParams}}
model_params["{{$k}}"]={{$v | attrToPythonValue}}
{{end}}

feature_columns = {{.FeatureColumnCode}}

train_max_steps = {{index .TrainParams "max_steps" | attrToPythonValue}}
train_max_steps = None if train_max_steps == 0 else train_max_steps


train(is_keras_model="{{.IsKerasModel}}" == "true",
    datasource="{{.DataSource}}",
    estimator={{.Estimator}},
    select="""{{.TrainSelect}}""",
    validate_select="""{{.ValidationSelect}}""",
    feature_columns=feature_columns,
    feature_column_names=feature_column_names,
    feature_metas=feature_metas,
    label_meta=label_meta,
    model_params=model_params,
    save="{{.Save}}",
    batch_size=1,
    epochs={{index .TrainParams "epoch" | attrToPythonValue}},
    verbose={{index .TrainParams "verbose" | attrToPythonValue}},
    train_max_steps=train_max_steps,
    eval_start_delay_secs={{index .ValidationParams "start_delay_secs" | attrToPythonValue}},
    eval_throttle_secs={{index .ValidationParams "throttle_secs" | attrToPythonValue}},
    save_checkpoints_steps={{index .TrainParams "save_checkpoints_steps" | attrToPythonValue}},
    log_every_n_iter={{index .TrainParams "log_every_n_iter" | attrToPythonValue}})
`
