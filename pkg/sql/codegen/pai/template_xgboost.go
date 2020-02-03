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

type xgbSaveModelFiller struct {
	OSSModelDir string
}

const xgbSaveModelTmplText = `
from sqlflow_submitter.pai import model
# NOTE(typhoonzero): the xgboost model file "my_model" is hard coded in xgboost/train.py
model.save_file("{{.OSSModelDir}}", "my_model")
model.save_metas("{{.OSSModelDir}}",
		   1,
		   "xgboost_model_desc",
		   "", # estimator = ""
           model_params,
           train_params,
           feature_field_meta,
           label_field_meta)
`

const xgbLoadModelTmplText = `

`

const predTemplateText = `
import json
from sqlflow_submitter.xgboost.predict import pred
from sqlflow_submitter.pai import model

# NOTE(typhoonzero): the xgboost model file "my_model" is hard coded in xgboost/train.py
model.load_file("{{.OSSModelDir}}", "my_model")
estimator, model_params, train_params,
feature_field_meta, label_field_meta = model.load_metas("{{.OSSModelDir}}", "xgboost_model_desc")


pred(datasource='''{{.DataSource}}''',
     select='''{{.PredSelect}}''',
     feature_field_meta=feature_field_meta,
     label_field_meta=label_field_meta,
     result_table='''{{.ResultTable}}''',
     hdfs_namenode_addr='''{{.HDFSNameNodeAddr}}''',
     hive_location='''{{.HiveLocation}}''',
     hdfs_user='''{{.HDFSUser}}''',
     hdfs_pass='''{{.HDFSPass}}''')
`
