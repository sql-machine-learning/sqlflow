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
	HDFSNameNodeAddr string
	HiveLocation     string
	HDFSUser         string
	HDFSPass         string
	PAIPredictTable  string
}

const xgbPredTemplateText = `
import json
from sqlflow_submitter.xgboost.predict import pred
from sqlflow_submitter.pai import model
from sqlflow_submitter.tensorflow.pai_distributed import define_tf_flags, set_oss_environs

FLAGS = define_tf_flags()
set_oss_environs(FLAGS)

# NOTE(typhoonzero): the xgboost model file "my_model" is hard coded in xgboost/train.py
model.load_file("{{.OSSModelDir}}", "my_model")
(estimator,
model_params,
train_params,
feature_metas,
feature_column_names,
label_meta) = model.load_metas("{{.OSSModelDir}}", "xgboost_model_desc")

pred(datasource='''{{.DataSource}}''',
    select='''{{.PredSelect}}''',
    feature_metas=feature_metas,
    feature_column_names=feature_column_names,
    label_meta=label_meta,
    result_table='''{{.ResultTable}}''',
    is_pai=True,
    hdfs_namenode_addr='''{{.HDFSNameNodeAddr}}''',
    hive_location='''{{.HiveLocation}}''',
    hdfs_user='''{{.HDFSUser}}''',
    hdfs_pass='''{{.HDFSPass}}''',
    pai_table='''{{.PAIPredictTable}}''',
    model_params=model_params,
    train_params=train_params)
`

type xgbExplainFiller struct {
	OSSModelDir       string
	DataSource        string
	DatasetSQL        string
	ResultTable       string
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
if os.environ.get('DISPLAY', '') == '':
    print('no display found. Using non-interactive Agg backend')
    matplotlib.use('Agg')

import json
from sqlflow_submitter.xgboost.explain import explain
from sqlflow_submitter.pai import model
from sqlflow_submitter.tensorflow.pai_distributed import define_tf_flags, set_oss_environs

FLAGS = define_tf_flags()
set_oss_environs(FLAGS)

# NOTE(typhoonzero): the xgboost model file "my_model" is hard coded in xgboost/train.py
model.load_file("{{.OSSModelDir}}", "my_model")

(estimator,
model_params,
train_params,
feature_field_meta,
feature_column_names,
label_field_meta) = model.load_metas("{{.OSSModelDir}}", "xgboost_model_desc")

explain(
    datasource='''{{.DataSource}}''',
    select='''{{.DatasetSQL}}''',
	feature_field_meta=feature_field_meta,
	feature_column_names=feature_column_names,
	label_spec=label_field_meta,
	summary_params={},
	result_table='''{{.ResultTable}}''',
	is_pai="{{.IsPAI}}" == "true",
	pai_explain_table="{{.PAIExplainTable}}",
	hdfs_namenode_addr='''{{.HDFSNameNodeAddr}}''',
	hive_location='''{{.HiveLocation}}''',
	hdfs_user='''{{.HDFSUser}}''',
	hdfs_pass='''{{.HDFSPass}}''',
	oss_dest='''{{.ResultOSSDest}}''',
	oss_ak='''{{.ResultOSSAK}}''',
	oss_sk='''{{.ResultOSSSK}}''',
	oss_endpoint='''{{.ResultOSSEndpoint}}''',
	oss_bucket_name='''{{.ResultOSSBucket}}''')
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
from sqlflow_submitter.xgboost.evaluate import evaluate
from sqlflow_submitter.pai import model
from sqlflow_submitter.tensorflow.pai_distributed import define_tf_flags, set_oss_environs

FLAGS = define_tf_flags()
set_oss_environs(FLAGS)

# NOTE(typhoonzero): the xgboost model file "my_model" is hard coded in xgboost/train.py
model.load_file("{{.OSSModelDir}}", "my_model")
(estimator,
model_params,
train_params,
feature_metas,
feature_column_names,
label_meta) = model.load_metas("{{.OSSModelDir}}", "xgboost_model_desc")

evaluate(datasource='''{{.DataSource}}''',
         select='''{{.PredSelect}}''',
         feature_metas=feature_metas,
         feature_column_names=feature_column_names,
         label_meta=label_meta,
         result_table='''{{.ResultTable}}''',
         validation_metrics="{{.MetricNames}}".split(","), 
         hdfs_namenode_addr='''{{.HDFSNameNodeAddr}}''',
         hive_location='''{{.HiveLocation}}''',
         hdfs_user='''{{.HDFSUser}}''',
         hdfs_pass='''{{.HDFSPass}}''',
         is_pai=True,
         pai_table="{{.PAIEvaluateTable}}",
         model_params=model_params)
`
