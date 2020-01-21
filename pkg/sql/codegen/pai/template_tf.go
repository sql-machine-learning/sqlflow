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

type wrapperFiller struct {
	ClusterConfigJSON string
	IsDistributed     bool
	DataSource        string
	EntryFile         string
	ModelName         string
	PAITrainTable     string
	PAIValidateTable  string
	ResultTable       string
	OSSCheckpointDir  string // uri for PAI to save checkpoints on OSS, e.g. oss://bucket/dir/?role_arn=xxx&host=xxx
	IsXgboost         bool
}

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

const tfWrapperTmplText = `
import os
import subprocess
import tarfile

import sqlflow_submitter.db

tarball = "/%s/sqlflow_model.tar.gz" % os.getcwd()
archive = tarfile.open(tarball, "w|gz")

with open("requirements.txt", "w") as req_fn:
    req_fn.write("shap==0.28.5\n")
    req_fn.write("seaborn==0.9.0\n")
{{if .IsXgboost}}
    req_fn.write("xgboost==0.82\n")
{{end}}

# '.' is always in sys.path
archive.add(sqlflow_submitter.__path__[0], arcname='sqlflow_submitter')
archive.add('{{.EntryFile}}')
archive.add('requirements.txt')
archive.close()

driver, dsn = "{{.DataSource}}".split("://")
assert driver == "maxcompute"
user, passwd, address, database = sqlflow_submitter.db.parseMaxComputeDSN(dsn)

jobname = '_'.join(['sqlflow', '{{.ModelName}}'.replace('.', '_')])
# The tags candidate list is: "cnn,dnn,rnn,bert,ctr,cvr,inception,resnet,gnn,gcn,ocr,maskrcnn,transformer,nmt,others". Use "others" if you are not sure which tags you need.
# Need to add arguments -Dbuckets="oss://..." -DcheckpointDir="oss://..." for distributed training.

train_table_parts = "{{.PAITrainTable}}".split(".")
val_table = "{{.PAIValidateTable}}"
if val_table == "":
    submit_tables = "odps://%s/tables/%s" % (train_table_parts[0], train_table_parts[1])
else:
    val_table_parts = val_table.split(".")
    submit_tables = "odps://%s/tables/%s,odps://%s/tables/%s" % (train_table_parts[0], train_table_parts[1], val_table_parts[0], val_table_parts[1])

if "{{.ResultTable}}" != "":
    result_table_parts = "{{.ResultTable}}".split(".")
    submit_result_tables = "-Doutputs=odps://%s/tables/%s" % (result_table_parts[0], result_table_parts[1])
else:
    # when training we do not need write result to a table.
    submit_result_tables = ""

print("saving model to: {{.OSSCheckpointDir}}")
{{ if .IsDistributed }}
pai_cmd = "pai -name %s -DjobName=%s -Dtags=%s -Dscript=file://%s -DentryFile=%s -Dtables=%s %s -DcheckpointDir=\"{{.OSSCheckpointDir}}\" -Dcluster=\"%s\"" % (
    'tensorflow1120', jobname, 'dnn', tarball, '{{.EntryFile}}', submit_tables, submit_result_tables, {{.ClusterConfigJSON}}.replace("\"", "\\\""))
{{else}}
pai_cmd = "pai -name %s -DjobName=%s -Dtags=%s -Dscript=file://%s -DentryFile=%s -DgpuRequired=\"0\" -Dtables=%s %s -DcheckpointDir=\"{{.OSSCheckpointDir}}\"" % (
    'tensorflow1120', jobname, 'dnn', tarball, '{{.EntryFile}}', submit_tables, submit_result_tables)
{{end}}

# Submit the tarball to PAI
subprocess.run(["odpscmd", "-u", user,
                           "-p", passwd,
                           "--project", database,
                           "--endpoint", address,
                           "-e", pai_cmd],
               check=True)
`

const tfSaveModelTmplText = `
from sqlflow_submitter.pai import model
model.save("{{.OSSModelDir}}",
           {{.NumWorkers}},
           "{{.Estimator}}",
           feature_column_names,
           feature_metas,
           label_meta,
           model_params,
           feature_columns)
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
 feature_columns) = model.load("{{.OSSModelDir}}")

predict.pred(datasource="{{.DataSource}}",
             estimator=eval(estimator),
             select="""{{.Select}}""",
             result_table="{{.ResultTable}}",
             feature_columns=feature_columns,
             feature_column_names=feature_column_names,
             feature_metas=feature_metas,
             label_meta=label_meta,
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
feature_columns) = model.load("{{.OSSModelDir}}")
 
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
