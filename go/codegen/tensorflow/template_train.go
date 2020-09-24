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

package tensorflow

import "sqlflow.org/sqlflow/go/ir"

type trainFiller struct {
	DataSource          string
	TrainSelect         string
	ValidationSelect    string
	Estimator           string
	FieldDescs          map[string][]*ir.FieldDesc
	FeatureColumnCode   string
	Y                   *ir.FieldDesc
	ModelParams         map[string]interface{}
	TrainParams         map[string]interface{}
	ValidationParams    map[string]interface{}
	Save                string
	OSSModelDirToLoad   string
	LoadPreTrainedModel bool
	IsPAI               bool
	PAITrainTable       string
	PAIValidateTable    string
	ModelRepoImage      string
	OriginalSQL         string
}

const tfTrainTemplateText = `# -*- coding: utf-8 -*-
import copy
import traceback
import tensorflow as tf
import runtime
{{ if .IsPAI }}
from runtime.pai.tensorflow_submitter.train import train
{{ else }}
from runtime.tensorflow.train import train
{{ end }}
from runtime.tensorflow.get_tf_version import tf_is_version2
from tensorflow.estimator import (DNNClassifier,
                                  DNNRegressor,
                                  LinearClassifier,
                                  LinearRegressor,
                                  BoostedTreesClassifier,
                                  BoostedTreesRegressor,
                                  DNNLinearCombinedClassifier,
                                  DNNLinearCombinedRegressor)
if tf_is_version2():
    from tensorflow.keras.optimizers import Adadelta, Adagrad, Adam, Adamax, Ftrl, Nadam, RMSprop, SGD
    from tensorflow.keras.losses import BinaryCrossentropy, CategoricalCrossentropy, CategoricalHinge, CosineSimilarity, Hinge, Huber, KLDivergence, LogCosh, MeanAbsoluteError, MeanAbsolutePercentageError, MeanSquaredError, MeanSquaredLogarithmicError, Poisson, SparseCategoricalCrossentropy, SquaredHinge
else:
    from tensorflow.train import AdadeltaOptimizer, AdagradOptimizer, AdamOptimizer, FtrlOptimizer, RMSPropOptimizer, GradientDescentOptimizer, MomentumOptimizer
    from tensorflow.keras.losses import BinaryCrossentropy, CategoricalCrossentropy, CategoricalHinge, CosineSimilarity, Hinge, Huber, KLDivergence, LogCosh, MeanAbsoluteError, MeanAbsolutePercentageError, MeanSquaredError, MeanSquaredLogarithmicError, Poisson, SparseCategoricalCrossentropy, SquaredHinge
try:
    import sqlflow_models
except Exception as e:
    print("failed to import sqlflow_models: %s", e)
    traceback.print_exc()

feature_column_names = [{{range $target, $desclist := .FieldDescs}}{{range $desclist}}
"{{.Name}}",
{{end}}{{end}}]

# feature_column_names_map is used to determine the order of feature columns of each target:
# e.g. when using DNNLinearCombinedClassifer.
# feature_column_names_map will be saved to a single file when using PAI.
feature_column_names_map = dict()
{{range $target, $desclist := .FieldDescs}}
feature_column_names_map["{{$target}}"] = [{{range $desclist}}"{{.Name}}",{{end}}]
{{end}}

feature_metas = dict()
{{ range $target, $desclist := .FieldDescs }}
{{ range $value := $desclist }}
feature_metas["{{$value.Name}}"] = {
    "feature_name": "{{$value.Name}}",
    "dtype": "{{$value.DType | DTypeToString}}",
    "delimiter": "{{$value.Delimiter}}",
    "format": "{{$value.Format}}",
    "shape": {{$value.Shape | intArrayToJSONString}},
    "is_sparse": "{{$value.IsSparse}}" == "true",
    "dtype_weight": "{{$value.DTypeWeight | DTypeToString}}",
    "delimiter_kv": "{{$value.DelimiterKV}}"
}
{{end}}
{{end}}

label_meta = {
    "feature_name": "{{.Y.Name}}",
    "dtype": "{{.Y.DType | DTypeToString}}",
    "delimiter": "{{.Y.Delimiter}}",
    "shape": {{.Y.Shape | intArrayToJSONString}},
    "is_sparse": "{{.Y.IsSparse}}" == "true"
}

model_params=dict()
{{range $k, $v := .ModelParams}}
model_params["{{$k}}"]={{$v | attrToPythonValue}}
{{end}}

# Construct optimizer objects to pass to model initializer.
# The original model_params is serializable (do not have tf.xxx objects).
model_params_constructed = copy.deepcopy(model_params)
for optimizer_arg in ["optimizer", "dnn_optimizer", "linear_optimizer"]:
    if optimizer_arg in model_params_constructed:
        model_params_constructed[optimizer_arg] = eval(model_params_constructed[optimizer_arg])

if "loss" in model_params_constructed:
    model_params_constructed["loss"] = eval(model_params_constructed["loss"])

# feature_columns_code will be used to save the training informations together
# with the saved model.
feature_columns_code = """{{.FeatureColumnCode}}"""
feature_columns = eval(feature_columns_code)

train_max_steps = {{index .TrainParams "max_steps" | attrToPythonValue}}
train_max_steps = None if train_max_steps == 0 else train_max_steps

train(datasource="{{.DataSource}}",
      estimator_string="""{{.Estimator}}""",
      select="""{{.TrainSelect}}""",
      validation_select="""{{.ValidationSelect}}""",
      feature_columns=feature_columns,
      feature_column_names=feature_column_names,
      feature_metas=feature_metas,
      label_meta=label_meta,
      model_params=model_params_constructed,
      validation_metrics="{{index .ValidationParams "metrics"}}".split(","),
      save="{{.Save}}",
      batch_size={{index .TrainParams "batch_size" | attrToPythonValue}},
      epoch={{index .TrainParams "epoch" | attrToPythonValue}},
      validation_steps={{index .ValidationParams "steps" | attrToPythonValue}},
      verbose={{index .TrainParams "verbose" | attrToPythonValue}},
      max_steps=train_max_steps,
      validation_start_delay_secs={{index .ValidationParams "start_delay_secs" | attrToPythonValue}},
      validation_throttle_secs={{index .ValidationParams "throttle_secs" | attrToPythonValue}},
      save_checkpoints_steps={{index .TrainParams "save_checkpoints_steps" | attrToPythonValue}},
      log_every_n_iter={{index .TrainParams "log_every_n_iter" | attrToPythonValue}},
      load_pretrained_model="{{.LoadPreTrainedModel}}" == "true",
      is_pai="{{.IsPAI}}" == "true",
      pai_table="{{.PAITrainTable}}",
      pai_val_table="{{.PAIValidateTable}}",
      feature_columns_code=feature_columns_code,
      model_params_code_map=model_params,
      model_repo_image="{{.ModelRepoImage}}",
      original_sql='''{{.OriginalSQL}}''',
      feature_column_names_map=feature_column_names_map)
`
