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
}

type explainFiller struct {
	OSSModelDir string
	DataSource  string
	Select      string
	ResultTable string
	IsPAI       bool
	PAITable    string
}

type requirementsFiller struct {
	IsXGBoost bool
}

const tfSaveModelTmplText = `
from sqlflow_submitter.pai import model
model.save_metas("{{.OSSModelDir}}",
           {{.NumWorkers}},
           "tensorflow_model_desc",
           "{{.Estimator}}",
           feature_column_names,
           feature_metas,
           label_meta,
           model_params,
           feature_columns)
`

const paiRequirementsTmplText = `
shap==0.28.5
seaborn==0.9.0
adanet==0.8.0
pandas==0.24.2
numpy==1.16.2
scikit-learn==0.20.0
{{if .IsXGBoost }}
xgboost==0.82
{{end}}
`

const tfPredictTmplText = `
import tensorflow as tf
from tensorflow.estimator import DNNClassifier, DNNRegressor, LinearClassifier, LinearRegressor, BoostedTreesClassifier, BoostedTreesRegressor, DNNLinearCombinedClassifier, DNNLinearCombinedRegressor
from sqlflow_submitter.pai import model
from sqlflow_submitter.tensorflow import predict
try:
    tf.enable_eager_execution()
except:
    pass

(estimator,
 feature_column_names,
 feature_metas,
 label_meta,
 model_params,
 feature_columns) = model.load_metas("{{.OSSModelDir}}", "tensorflow_model_desc")

predict.pred(datasource="{{.DataSource}}",
             estimator=eval(estimator),
             select="""{{.Select}}""",
             result_table="{{.ResultTable}}",
             feature_columns=feature_columns,
             feature_column_names=feature_column_names,
             feature_metas=feature_metas,
             model_params=model_params,
             save="{{.OSSModelDir}}",
             batch_size=1,
             is_pai="{{.IsPAI}}" == "true",
             pai_table="{{.PAITable}}")
`

const tfExplainTmplText = `
import json
import sys
import tensorflow as tf
from tensorflow.estimator import DNNClassifier, DNNRegressor, LinearClassifier, LinearRegressor, BoostedTreesClassifier, BoostedTreesRegressor, DNNLinearCombinedClassifier, DNNLinearCombinedRegressor
from sqlflow_submitter.pai import model
from sqlflow_submitter.tensorflow import explain
try:
    tf.enable_eager_execution()
except Exception as e:
    sys.stderr.write("warning: failed to enable_eager_execution: %s" % e)
    pass

(estimator,
feature_column_names,
feature_metas,
label_meta,
model_params,
feature_columns) = model.load_metas("{{.OSSModelDir}}", "tensorflow_model_desc")
 
explain.explain(datasource="{{.DataSource}}",
                estimator_cls=eval(estimator),
                select="""{{.Select}}""",
                feature_columns=feature_columns,
                feature_column_names=feature_column_names,
                feature_metas=feature_metas,
                label_meta=label_meta,
                model_params=model_params,
                save="{{.OSSModelDir}}",
                result_table="{{.ResultTable}}",
                is_pai="{{.IsPAI}}" == "true",
                pai_table="{{.PAITable}}")
`
