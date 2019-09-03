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

package sql

const xgbTrainTemplateText = `
import xgboost as xgb
from sqlflow_submitter.db import connect, db_generator

driver="{{.Driver}}"

{{if ne .Database ""}}
database="{{.Database}}"
{{else}}
database=""
{{end}}

session_cfg = {}
{{ range $k, $v := .Session }}
session_cfg["{{$k}}"] = "{{$v}}"
{{end}}

{{if ne .TrainCfgJSON ""}}
train_args = {{.TrainCfgJSON}}
{{else}}
train_args = {}
{{end}}

{{if ne .ParamsCfgJSON ""}}
params = {{.ParamsCfgJSON}}
{{else}}
params = {}
{{end}}

feature_column_names = [{{range .Features}}
"{{.FeatureName}}",
{{end}}]

{{/* Convert go side featureSpec to python dict for input_fn */}}
feature_specs = dict()
{{ range $value := .Features }}
feature_specs["{{$value.FeatureName}}"] = {
    "feature_name": "{{$value.FeatureName}}",
    "dtype": "{{$value.Dtype}}",
    "delimiter": "{{$value.Delimiter}}",
    "shape": {{$value.InputShape}},
    "is_sparse": "{{$value.IsSparse}}" == "true"
}
{{end}}



conn = connect(driver, database, user="{{.User}}", password="{{.Password}}", host="{{.Host}}", port={{.Port}}, auth="{{.Auth}}")

def xgb_dataset(fn, dataset_sql):
		gen = db_generator(driver, conn, session_cfg, dataset_sql, feature_column_names, "{{.Label.FeatureName}}", feature_specs)
		with open(fn, 'w') as f:
				for item in gen():
						features, label = item
						row_data = [str(label[0])] + ["%d:%f" % (i, v) for i, v in enumerate(features)]
						f.write("\t".join(row_data) + "\n")
		# TODO(yancey1989): genearte group and weight text file if necessary
		return xgb.DMatrix(fn) 

dtrain = xgb_dataset('train.txt', "{{.TrainingDatasetSQL}}")
dtest = xgb_dataset('test.txt', "{{.ValidationDatasetSQL}}")

#TODO(Yancey1989): specify the eval metrics by WITH statement in SQL
train_args["evals"] = [(dtest, "auc")]
bst = xgb.train(params, dtrain, **train_args)
bst.save_model()
`
