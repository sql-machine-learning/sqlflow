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

type xgbPredictFiller struct {
	OSSModelDir      string
	DataSource       string
	PredSelect       string
	ResultTable      string
	ResultColumn     string
	HDFSNameNodeAddr string
	HiveLocation     string
	HDFSUser         string
	HDFSPass         string
	PAIPredictTable  string
}

const xgbPredTemplateText = `
import json
import copy
import runtime.xgboost as xgboost_extended
from runtime.xgboost.predict import pred
from runtime.model import oss
from runtime.pai.pai_distributed import define_tf_flags, set_oss_environs

FLAGS = define_tf_flags()
set_oss_environs(FLAGS)

# NOTE(typhoonzero): the xgboost model file "my_model" is hard coded in xgboost/train.py
oss.load_file("{{.OSSModelDir}}", "my_model")
(estimator,
model_params,
train_params,
feature_metas,
feature_column_names,
label_meta,
feature_column_code) = oss.load_metas("{{.OSSModelDir}}", "xgboost_model_desc")

pred_label_meta = copy.copy(label_meta)
pred_label_meta["feature_name"] = "{{.ResultColumn}}"

feature_column_transformers = eval('[{}]'.format(feature_column_code))
transform_fn = xgboost_extended.feature_column.ComposedColumnTransformer(feature_column_names, *feature_column_transformers)

pred(datasource='''{{.DataSource}}''',
    select='''{{.PredSelect}}''',
    feature_metas=feature_metas,
    feature_column_names=feature_column_names,
    train_label_meta=label_meta,
    pred_label_meta=label_meta,
    result_table='''{{.ResultTable}}''',
    is_pai=True,
    pai_table='''{{.PAIPredictTable}}''',
    model_params=model_params,
    train_params=train_params,
    transform_fn=transform_fn,
    feature_column_code=feature_column_code,
    flags=FLAGS)
`

type xgbExplainFiller struct {
	OSSModelDir       string
	DataSource        string
	DatasetSQL        string
	ResultTable       string
	Explainer         string
	IsPAI             bool
	PAIExplainTable   string
	HDFSNameNodeAddr  string
	HiveLocation      string
	HDFSUser          string
	HDFSPass          string
	ResultOSSDest     string
	ResultOSSAK       string
	ResultOSSSK       string
	ResultOSSEndpoint string
	ResultOSSBucket   string
}

const xgbExplainTemplateText = `
# Running on PAI
import os
import matplotlib
import runtime.xgboost as xgboost_extended

if os.environ.get('DISPLAY', '') == '':
    print('no display found. Using non-interactive Agg backend')
    matplotlib.use('Agg')

import json
from runtime.xgboost.explain import explain
from runtime.model import oss
from runtime.pai.pai_distributed import define_tf_flags, set_oss_environs

FLAGS = define_tf_flags()
set_oss_environs(FLAGS)

# NOTE(typhoonzero): the xgboost model file "my_model" is hard coded in xgboost/train.py
oss.load_file("{{.OSSModelDir}}", "my_model")

(estimator,
model_params,
train_params,
feature_field_meta,
feature_column_names,
label_field_meta,
feature_column_code) = oss.load_metas("{{.OSSModelDir}}", "xgboost_model_desc")

feature_column_transformers = eval('[{}]'.format(feature_column_code))
transform_fn = xgboost_extended.feature_column.ComposedColumnTransformer(feature_column_names, *feature_column_transformers)

explain(
    datasource='''{{.DataSource}}''',
    select='''{{.DatasetSQL}}''',
	feature_field_meta=feature_field_meta,
	feature_column_names=feature_column_names,
	label_meta=label_field_meta,
	summary_params={},
	explainer="{{.Explainer}}",
	result_table='''{{.ResultTable}}''',
	is_pai="{{.IsPAI}}" == "true",
	pai_explain_table="{{.PAIExplainTable}}",
	oss_dest='''{{.ResultOSSDest}}''',
	oss_ak='''{{.ResultOSSAK}}''',
	oss_sk='''{{.ResultOSSSK}}''',
	oss_endpoint='''{{.ResultOSSEndpoint}}''',
	oss_bucket_name='''{{.ResultOSSBucket}}''',
	transform_fn=transform_fn,
	feature_column_code=feature_column_code)
`

type xgbEvaluateFiller struct {
	OSSModelDir      string
	DataSource       string
	PredSelect       string
	ResultTable      string
	MetricNames      string
	HDFSNameNodeAddr string
	HiveLocation     string
	HDFSUser         string
	HDFSPass         string
	PAIEvaluateTable string
}

const xgbEvalTemplateText = `
import json
import runtime.xgboost as xgboost_extended
from runtime.xgboost.evaluate import evaluate
from runtime.model import oss
from runtime.pai.pai_distributed import define_tf_flags, set_oss_environs

FLAGS = define_tf_flags()
set_oss_environs(FLAGS)

# NOTE(typhoonzero): the xgboost model file "my_model" is hard coded in xgboost/train.py
oss.load_file("{{.OSSModelDir}}", "my_model")
(estimator,
model_params,
train_params,
feature_metas,
feature_column_names,
label_meta,
feature_column_code) = oss.load_metas("{{.OSSModelDir}}", "xgboost_model_desc")

feature_column_transformers = eval('[{}]'.format(feature_column_code))
transform_fn = xgboost_extended.feature_column.ComposedColumnTransformer(feature_column_names, *feature_column_transformers)

evaluate(datasource='''{{.DataSource}}''',
         select='''{{.PredSelect}}''',
         feature_metas=feature_metas,
         feature_column_names=feature_column_names,
         label_meta=label_meta,
         result_table='''{{.ResultTable}}''',
         validation_metrics="{{.MetricNames}}".split(","),
         is_pai=True,
         pai_table="{{.PAIEvaluateTable}}",
         model_params=model_params,
         transform_fn=transform_fn,
         feature_column_code=feature_column_code)
`
