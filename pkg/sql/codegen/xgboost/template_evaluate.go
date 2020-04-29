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

import (
	"text/template"
)

type evalFiller struct {
	DataSource         string
	PredSelect         string
	FeatureMetaJSON    string
	LabelMetaJSON      string
	FeatureColumnNames []string
	MetricNames        string
	ResultTable        string
	HDFSNameNodeAddr   string
	HiveLocation       string
	HDFSUser           string
	HDFSPass           string
	IsPAI              bool
	PAITable           string
}

const evalTemplateText = `
from sqlflow_submitter.xgboost.evaluate import evaluate
from sqlflow_submitter.tensorflow.pai_distributed import define_tf_flags, set_oss_environs
import json

FLAGS = define_tf_flags()
set_oss_environs(FLAGS)

feature_metas = json.loads('''{{.FeatureMetaJSON}}''')
label_meta = json.loads('''{{.LabelMetaJSON}}''')

feature_column_names = [{{range .FeatureColumnNames}}
"{{.}}",
{{end}}]

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
         is_pai="{{.IsPAI}}" == "true",
         pai_table="{{.PAITable}}")
`

var evalTemplate = template.Must(template.New("Eval").Parse(evalTemplateText))
