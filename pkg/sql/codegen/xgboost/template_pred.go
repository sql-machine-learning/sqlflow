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

package xgboost

import (
	"text/template"
)

type predFiller struct {
	DataSource       string
	PredSelect       string
	FeatureMetaJSON  string
	LabelMetaJSON    string
	ResultTable      string
	HDFSNameNodeAddr string
	HiveLocation     string
	HDFSUser         string
	HDFSPass         string
}

const predTemplateText = `
import json
from sqlflow_submitter.xgboost.predict import pred

feature_field_meta = json.loads('''{{.FeatureMetaJSON}}''')
label_field_meta = json.loads('''{{.LabelMetaJSON}}''')

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

var predTemplate = template.Must(template.New("Pred").Parse(predTemplateText))
