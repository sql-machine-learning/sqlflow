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

type explainFiller struct {
	DataSource    string
	Select        string
	SummaryParams string
	// below members comes from trainStmt
	EstimatorClass    string
	IsKerasModel      bool
	FieldDescs        []*ir.FieldDesc
	FeatureColumnCode string
	Y                 *ir.FieldDesc
	ModelParams       map[string]interface{}
	Save              string
	IsPAI             bool
	ResultTable       string
	HDFSNameNodeAddr  string
	HiveLocation      string
	HDFSUser          string
	HDFSPass          string
}

const boostedTreesExplainTemplateText = `
from sqlflow_submitter.tensorflow import explain
import tensorflow as tf
try:
    import sqlflow_models
except:
    pass

feature_column_names = [{{range .FieldDescs}}
"{{.Name}}",
{{end}}]

feature_metas = dict()
{{ range $value := .FieldDescs }}
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

import json
summaryAttrs = json.loads('''{{.SummaryParams}}''')

explain.explain(datasource="{{.DataSource}}",
                estimator_cls={{.EstimatorClass}},
                select="""{{.Select}}""",
                feature_columns=feature_columns,
                feature_column_names=feature_column_names,
                feature_metas=feature_metas,
                label_meta=label_meta,
                model_params=model_params,
                save="{{.Save}}",
                result_table="{{.ResultTable}}",
                hdfs_namenode_addr="{{.HDFSNameNodeAddr}}",
                hive_location="{{.HiveLocation}}",
                hdfs_user="{{.HDFSUser}}",
                hdfs_pass="{{.HDFSPass}}",
                **summaryAttrs)
`
