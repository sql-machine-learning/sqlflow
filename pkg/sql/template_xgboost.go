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

num_boost_round = {{.NumBoostRound}}
maximize = True if "{{.Maximize}}" == "true" else False
early_stopping_rounds = {{.EarlyStoppingRounds}}
if early_stopping_rounds == -1:
    early_stopping_rounds = None

{{if ne .ParamsCfgJSON ""}}
params = {{.ParamsCfgJSON}}
{{else}}
params = {}
{{end}}

feature_column_names = [{{range .X}}
"{{.FeatureName}}",
{{end}}]

{{/* Convert go side featureSpec to python dict for input_fn */}}
feature_specs = dict()
{{ range $value := .X }}
feature_specs["{{$value.FeatureName}}"] = {
    "feature_name": "{{$value.FeatureName}}",
    "dtype": "{{$value.Dtype}}",
    "delimiter": "{{$value.Delimiter}}",
    "shape": {{$value.InputShape}},
    "is_sparse": "{{$value.IsSparse}}" == "true"
}
{{end}}

conn = connect(driver, database, user="{{.User}}", password="{{.Password}}", host="{{.Host}}", port={{.Port}}, auth="{{.Auth}}",session_cfg=session_cfg)

def xgb_dataset(fn, dataset_sql):
    gen = db_generator(driver, conn, dataset_sql, feature_column_names, "{{.Y.FeatureName}}", feature_specs)
    with open(fn, 'w') as f:
        for item in gen():
            features, label = item
            row_data = [str(label[0])] + ["%d:%f" % (i, v) for i, v in enumerate(features)]
            f.write("\t".join(row_data) + "\n")
    # TODO(yancey1989): genearte group and weight text file if necessary
    return xgb.DMatrix(fn)

dtrain = xgb_dataset('train.txt', """{{.TrainingDatasetSQL}}""")
dtest = xgb_dataset('test.txt', """{{.ValidationDatasetSQL}}""")

train_args = {}
train_args["num_boost_round"] = num_boost_round
train_args["maximize"] = maximize
train_args["early_stopping_rounds"] = early_stopping_rounds
train_args["evals"] = [(dtrain, "train"), (dtest, "validation")]

bst = xgb.train(params, dtrain, **train_args)
bst.save_model("{{.Save}}")
`

const xgbPredictTemplateText = `
import xgboost as xgb
import numpy as np
from sqlflow_submitter.db import connect, db_generator, buffered_db_writer

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

feature_column_names = [{{range .X}}
"{{.FeatureName}}",
{{end}}]

{{/* Convert go side featureSpec to python dict for input_fn */}}
feature_specs = dict()
{{ range $value := .X }}
feature_specs["{{$value.FeatureName}}"] = {
    "feature_name": "{{$value.FeatureName}}",
    "dtype": "{{$value.Dtype}}",
    "delimiter": "{{$value.Delimiter}}",
    "shape": {{$value.InputShape}},
    "is_sparse": "{{$value.IsSparse}}" == "true"
}
{{end}}

conn = connect(driver, database, user="{{.User}}", password="{{.Password}}", host="{{.Host}}", port={{.Port}}, auth="{{.Auth}}",session_cfg=session_cfg)

def xgb_dataset(fn, dataset_sql):
    gen = db_generator(driver, conn, dataset_sql, feature_column_names, "", feature_specs)
    with open(fn, 'w') as f:
        for item in gen():
            features, label = item
            row_data = [str(label[0])] + ["%d:%f" % (i, v) for i, v in enumerate(features)]
            f.write("\t".join(row_data) + "\n")
    # TODO(yancey1989): genearte group and weight text file if necessary
    return xgb.DMatrix(fn)

dpred = xgb_dataset('predict.txt', """{{.PredictionDatasetSQL}}""")

bst = xgb.Booster({'nthread': 4})  # init model
bst.load_model("{{.Save}}")  # load data
preds = bst.predict(dpred)

# TODO(Yancey1989): using the train parameters to decide regressoin model or classifier model
if len(preds.shape) == 2:
    # classifier result
    preds = np.argmax(np.array(preds), axis=1)

feature_file_read = open("predict.txt", "r")

result_column_names = feature_column_names
result_column_names.append("{{.Y.FeatureName}}")
line_no = 0
with buffered_db_writer(driver, conn, "{{.TableName}}", result_column_names, 100, hdfs_namenode_addr="{{.HDFSNameNodeAddr}}", hive_location="{{.HiveLocation}}") as w:
    while True:
        line = feature_file_read.readline()
        if not line:
            break
        row = [i.split(":")[1] for i in line.replace("\n", "").split("\t")[1:]]
        row.append(preds[line_no])
        w.write(row)
        line_no += 1
print("Done predicting. Predict table : {{.TableName}}")
`
