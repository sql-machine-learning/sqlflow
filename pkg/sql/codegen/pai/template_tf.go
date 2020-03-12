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

type requirementsFiller struct {
	IsXGBoost bool
}

const tfSaveModelTmplText = `
from sqlflow_submitter.pai import model
from shutil import copyfile
import types

estimator = {{.Estimator}}
if isinstance(estimator, types.FunctionType):
    is_estimator = False
else:
    is_estimator = issubclass(
        estimator,
        (tf.estimator.Estimator, tf.estimator.BoostedTreesClassifier,
            tf.estimator.BoostedTreesRegressor))

# Keras single node is using h5 format to save the model, no need to deal with export model format.
# Keras distributed mode will use estimator, so this is also needed.
if is_estimator or {{.NumWorkers}} > 1:
    FLAGS = tf.app.flags.FLAGS
    if FLAGS.task_index == 0:
        with open("exported_path", "r") as fn:
            saved_model_path = fn.read()
        model.save_dir("{{.OSSModelDir}}", saved_model_path)
        model.save_file("{{.OSSModelDir}}", "exported_path")

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
scikit-learn==0.20.0
{{if .IsXGBoost }}
xgboost==0.82
{{end}}
`

const tfPredictTmplText = `
import os
import types
import tensorflow as tf
from tensorflow.estimator import DNNClassifier, DNNRegressor, LinearClassifier, LinearRegressor, BoostedTreesClassifier, BoostedTreesRegressor, DNNLinearCombinedClassifier, DNNLinearCombinedRegressor
from sqlflow_submitter.pai import model
from sqlflow_submitter.tensorflow.pai_distributed import define_tf_flags, set_oss_environs
from sqlflow_submitter.tensorflow import predict
try:
    import sqlflow_models
except Exception as e:
    print("error importing sqlflow_models: %s" % e)
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

if isinstance(estimator, types.FunctionType):
    is_estimator = False
else:
    is_estimator = issubclass(
        eval(estimator),
        (tf.estimator.Estimator, tf.estimator.BoostedTreesClassifier,
            tf.estimator.BoostedTreesRegressor))

# Keras single node is using h5 format to save the model, no need to deal with export model format.
# Keras distributed mode will use estimator, so this is also needed.
if is_estimator:
    model.load_file("{{.OSSModelDir}}", "exported_path")
    # NOTE(typhoonzero): directory "model_save" is hardcoded in codegen/tensorflow/codegen.go
    model.load_dir("{{.OSSModelDir}}/model_save")
else:
    model.load_file("{{.OSSModelDir}}", "model_save")

predict.pred(datasource="{{.DataSource}}",
             estimator=eval(estimator),
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

const tfExplainTmplText = `
import os
import matplotlib
if os.environ.get('DISPLAY', '') == '':
	print('no display found. Using non-interactive Agg backend')
	matplotlib.use('Agg')

import json
import types
import sys
import tensorflow as tf
from tensorflow.estimator import DNNClassifier, DNNRegressor, LinearClassifier, LinearRegressor, BoostedTreesClassifier, BoostedTreesRegressor, DNNLinearCombinedClassifier, DNNLinearCombinedRegressor
from sqlflow_submitter.pai import model
from sqlflow_submitter.tensorflow.pai_distributed import define_tf_flags, set_oss_environs
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

if isinstance(estimator, types.FunctionType):
    is_estimator = False
else:
    is_estimator = issubclass(
        eval(estimator),
        (tf.estimator.Estimator, tf.estimator.BoostedTreesClassifier,
            tf.estimator.BoostedTreesRegressor))

# Keras single node is using h5 format to save the model, no need to deal with export model format.
# Keras distributed mode will use estimator, so this is also needed.
if is_estimator:
    model.load_file("{{.OSSModelDir}}", "exported_path")
    # NOTE(typhoonzero): directory "model_save" is hardcoded in codegen/tensorflow/codegen.go
    model.load_dir("{{.OSSModelDir}}/model_save")
else:
    model.load_file("{{.OSSModelDir}}", "model_save")

explain.explain(datasource="{{.DataSource}}",
                estimator_cls=eval(estimator),
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
