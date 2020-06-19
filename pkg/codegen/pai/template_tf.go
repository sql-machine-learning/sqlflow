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

package pai

type saveModelFiller struct {
	OSSModelDir string
	Estimator   string
	NumWorkers  int // used to determine whether is distributed training.
}

type loadModelFiller struct {
	OSSModelDir string
	Estimator   string
}

type predictFiller struct {
	OSSModelDir string
	DataSource  string
	Select      string
	ResultTable string
	IsPAI       bool
	PAITable    string
	Using       string
}

type explainFiller struct {
	OSSModelDir       string
	DataSource        string
	Select            string
	ResultTable       string
	IsPAI             bool
	PAITable          string
	ResultOSSDest     string
	ResultOSSAK       string
	ResultOSSSK       string
	ResultOSSEndpoint string
	ResultOSSBucket   string
}

type evaluateFiller struct {
	OSSModelDir string
	DataSource  string
	Select      string
	ResultTable string
	IsPAI       bool
	PAITable    string
	// validation metric names, e.g. "Accuracy,AUC"
	ValidationMetrics string
}

type requirementsFiller struct {
	IsXGBoost bool
}

const tfImportsText = `
import tensorflow as tf
from sqlflow_submitter.tensorflow import is_tf_estimator
from tensorflow.estimator import DNNClassifier, DNNRegressor, LinearClassifier, LinearRegressor, BoostedTreesClassifier, BoostedTreesRegressor, DNNLinearCombinedClassifier, DNNLinearCombinedRegressor
try:
	from sqlflow_submitter.pai import model
	from sqlflow_submitter.tensorflow.pai_distributed import define_tf_flags, set_oss_environs
except:
	pass # PAI is not always needed

`

const tfLoadModelTmplText = tfImportsText + `
FLAGS = define_tf_flags()
set_oss_environs(FLAGS)

estimator = {{.Estimator}}
is_estimator = is_tf_estimator(estimator)

# Keras single node is using h5 format to save the model, no need to deal with export model format.
# Keras distributed mode will use estimator, so this is also needed.
if is_estimator:
    model.load_file("{{.OSSModelDir}}", "exported_path")
    # NOTE(typhoonzero): directory "model_save" is hardcoded in codegen/tensorflow/codegen.go
    model.load_dir("{{.OSSModelDir}}/model_save")
else:
    model.load_file("{{.OSSModelDir}}", "model_save")
`

const tfSaveModelTmplText = tfImportsText + `
import types

estimator = {{.Estimator}}
is_estimator = is_tf_estimator(estimator)

# Keras single node is using h5 format to save the model, no need to deal with export model format.
# Keras distributed mode will use estimator, so this is also needed.
FLAGS = tf.app.flags.FLAGS
if is_estimator:
    if FLAGS.task_index == 0:
        with open("exported_path", "r") as fn:
            saved_model_path = fn.read()
        model.save_dir("{{.OSSModelDir}}", saved_model_path)
        model.save_file("{{.OSSModelDir}}", "exported_path")
else:
    if len(FLAGS.worker_hosts.split(",")) > 1:
        if FLAGS.task_index == 0:
            model.save_file("{{.OSSModelDir}}", "exported_path")
    else:
        model.save_file("{{.OSSModelDir}}", "model_save")

model.save_metas("{{.OSSModelDir}}",
           {{.NumWorkers}},
           "tensorflow_model_desc",
           "{{.Estimator}}",
           feature_column_names,
           feature_column_names_map,
           feature_metas,
           label_meta,
           model_params,
           feature_columns_code)
`

const paiRequirementsTmplText = `
adanet==0.8.0
numpy==1.16.2
pandas==0.24.2
plotille==3.7
seaborn==0.9.0
shap==0.28.5
scikit-learn==0.20.4
tensorflow-datasets==3.0.0
{{if .IsXGBoost }}
xgboost==0.82
sklearn2pmml==0.56.0
{{end}}
`

const tfPredictTmplText = tfImportsText + `
import os
import types
import traceback
from sqlflow_submitter.tensorflow import predict

try:
    import sqlflow_models
except Exception as e:
    print("error importing sqlflow_models: %s" % e)
    traceback.print_exc()
try:
    tf.enable_eager_execution()
except:
    pass

FLAGS = define_tf_flags()
set_oss_environs(FLAGS)

(estimator,
 feature_column_names,
 feature_column_names_map,
 feature_metas,
 label_meta,
 model_params,
 feature_columns_code) = model.load_metas("{{.OSSModelDir}}", "tensorflow_model_desc")

feature_columns = eval(feature_columns_code)

# NOTE(typhoonzero): No need to eval model_params["optimizer"] and model_params["loss"]
# because predicting do not need these parameters.

is_estimator = is_tf_estimator(eval(estimator))

# Keras single node is using h5 format to save the model, no need to deal with export model format.
# Keras distributed mode will use estimator, so this is also needed.
if is_estimator:
    model.load_file("{{.OSSModelDir}}", "exported_path")
    # NOTE(typhoonzero): directory "model_save" is hardcoded in codegen/tensorflow/codegen.go
    model.load_dir("{{.OSSModelDir}}/model_save")
else:
    model.load_file("{{.OSSModelDir}}", "model_save")

predict.pred(datasource="{{.DataSource}}",
             estimator_string=estimator,
             select="""{{.Select}}""",
             result_table="{{.ResultTable}}",
             feature_columns=feature_columns,
             feature_column_names=feature_column_names,
             feature_column_names_map=feature_column_names_map,
             result_col_name=label_meta["feature_name"],
             feature_metas=feature_metas,
             model_params=model_params,
             save="model_save",
             batch_size=1,
             is_pai="{{.IsPAI}}" == "true",
             pai_table="{{.PAITable}}")
`

const tfExplainTmplText = tfImportsText + `
import os
import matplotlib
if os.environ.get('DISPLAY', '') == '':
	print('no display found. Using non-interactive Agg backend')
	matplotlib.use('Agg')

import json
import types
import sys
from sqlflow_submitter.tensorflow import explain

try:
    tf.enable_eager_execution()
except Exception as e:
    sys.stderr.write("warning: failed to enable_eager_execution: %s" % e)
    pass

FLAGS = define_tf_flags()
set_oss_environs(FLAGS)

(estimator,
feature_column_names,
feature_column_names_map,
feature_metas,
label_meta,
model_params,
feature_columns_code) = model.load_metas("{{.OSSModelDir}}", "tensorflow_model_desc")

feature_columns = eval(feature_columns_code)
# NOTE(typhoonzero): No need to eval model_params["optimizer"] and model_params["loss"]
# because predicting do not need these parameters.

is_estimator = is_tf_estimator(eval(estimator))

# Keras single node is using h5 format to save the model, no need to deal with export model format.
# Keras distributed mode will use estimator, so this is also needed.
if is_estimator:
    model.load_file("{{.OSSModelDir}}", "exported_path")
    # NOTE(typhoonzero): directory "model_save" is hardcoded in codegen/tensorflow/codegen.go
    model.load_dir("{{.OSSModelDir}}/model_save")
else:
    model.load_file("{{.OSSModelDir}}", "model_save")


explain.explain(datasource="{{.DataSource}}",
                estimator_string=estimator,
                select="""{{.Select}}""",
                feature_columns=feature_columns,
                feature_column_names=feature_column_names,
                feature_metas=feature_metas,
                label_meta=label_meta,
                model_params=model_params,
                save="model_save",
                result_table="{{.ResultTable}}",
                is_pai="{{.IsPAI}}" == "true",
                pai_table="{{.PAITable}}",
                oss_dest='''{{.ResultOSSDest}}''',
                oss_ak='''{{.ResultOSSAK}}''',
                oss_sk='''{{.ResultOSSSK}}''',
                oss_endpoint='''{{.ResultOSSEndpoint}}''',
                oss_bucket_name='''{{.ResultOSSBucket}}''')
`

const tfEvaluateTmplText = tfImportsText + `
import os
import matplotlib
if os.environ.get('DISPLAY', '') == '':
	print('no display found. Using non-interactive Agg backend')
	matplotlib.use('Agg')

import json
import types
import sys
from sqlflow_submitter.tensorflow import evaluate

try:
    tf.enable_eager_execution()
except Exception as e:
    sys.stderr.write("warning: failed to enable_eager_execution: %s" % e)
    pass

FLAGS = define_tf_flags()
set_oss_environs(FLAGS)

(estimator,
feature_column_names,
feature_column_names_map,
feature_metas,
label_meta,
model_params,
feature_columns_code) = model.load_metas("{{.OSSModelDir}}", "tensorflow_model_desc")

feature_columns = eval(feature_columns_code)
# NOTE(typhoonzero): No need to eval model_params["optimizer"] and model_params["loss"]
# because predicting do not need these parameters.

is_estimator = is_tf_estimator(eval(estimator))

# Keras single node is using h5 format to save the model, no need to deal with export model format.
# Keras distributed mode will use estimator, so this is also needed.
if is_estimator:
    model.load_file("{{.OSSModelDir}}", "exported_path")
    # NOTE(typhoonzero): directory "model_save" is hardcoded in codegen/tensorflow/codegen.go
    model.load_dir("{{.OSSModelDir}}/model_save")
else:
    model.load_file("{{.OSSModelDir}}", "model_save")

evaluate.evaluate(datasource="{{.DataSource}}",
                  estimator_string=estimator,
                  select="""{{.Select}}""",
                  result_table="{{.ResultTable}}",
                  feature_columns=feature_columns,
                  feature_column_names=feature_column_names,
                  feature_metas=feature_metas,
                  label_meta=label_meta,
                  model_params=model_params,
                  validation_metrics="{{.ValidationMetrics}}".split(","),
                  save="model_save",
                  batch_size=1,
                  validation_steps=None,
                  verbose=0,
                  is_pai="{{.IsPAI}}" == "true",
                  pai_table="{{.PAITable}}")
`
