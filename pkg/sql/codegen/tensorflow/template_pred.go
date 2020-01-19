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

import "sqlflow.org/sqlflow/pkg/ir"

type predFiller struct {
	DataSource  string
	Select      string
	ResultTable string
	// below members comes from trainStmt
	Estimator         string
	FieldDescs        []*ir.FieldDesc
	FeatureColumnCode string
	Y                 *ir.FieldDesc
	ModelParams       map[string]interface{}
	Save              string
	HDFSNameNodeAddr  string
	HiveLocation      string
	HDFSUser          string
	HDFSPass          string
	IsPAI             bool
	PAIPredictTable   string
}

const tfPredTemplateText = `
import tensorflow as tf
from sqlflow_submitter.tensorflow.predict import pred
from sqlflow_submitter.tensorflow.train import TF_VERSION_2
from tensorflow.estimator import DNNClassifier, DNNRegressor, LinearClassifier, LinearRegressor, BoostedTreesClassifier, BoostedTreesRegressor, DNNLinearCombinedClassifier, DNNLinearCombinedRegressor
if TF_VERSION_2:
    from tensorflow.keras.optimizers import *
else:
    from tensorflow.train import *
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

pred(datasource="{{.DataSource}}",
     estimator={{.Estimator}},
     select="""{{.Select}}""",
     result_table="{{.ResultTable}}",
     feature_columns=feature_columns,
     feature_column_names=feature_column_names,
     result_col_name=label_meta["feature_name"],
     feature_metas=feature_metas,
     model_params=model_params,
     save="{{.Save}}",
     batch_size=1,
     hdfs_namenode_addr="{{.HDFSNameNodeAddr}}",
     hive_location="{{.HiveLocation}}",
     hdfs_user="{{.HDFSUser}}",
     hdfs_pass="{{.HDFSPass}}",
     is_pai="{{.IsPAI}}" == "true",
     pai_table="{{.PAIPredictTable}}")
`
