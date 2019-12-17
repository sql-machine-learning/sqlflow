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

package pai

type wrapperFiller struct {
	DataSource  string
	EntryFile   string
	ModelName   string
	NumPS       int
	NumWorkers  int // num_workers > 1 indicates we are running distributed training.
	PAIDatabase string
	PAITable    string
}

type saveModelFiller struct {
	DataSource   string
	ModelName    string
	Estimator    string
	Save         string
	IsKerasModel bool
}

type predictFiller struct {
	DataSource  string
	ModelName   string
	Select      string
	ResultTable string
}

const tfWrapperTmplText = `
import os
import subprocess
import tarfile

import sqlflow_submitter.db

tarball = "/%s/sqlflow_model.tar.gz" % os.getcwd()
archive = tarfile.open(tarball, "w|gz")

# '.' is always in sys.path
archive.add(sqlflow_submitter.__path__[0], arcname='sqlflow_submitter')
archive.add('{{.EntryFile}}')
archive.close()

driver, dsn = "{{.DataSource}}".split("://")
assert driver == "maxcompute"
user, passwd, address, database = sqlflow_submitter.db.parseMaxComputeDSN(dsn)

jobname = '_'.join(['sqlflow', '{{.ModelName}}'.replace('.', '_')])
# The tags candidate list is: "cnn,dnn,rnn,bert,ctr,cvr,inception,resnet,gnn,gcn,ocr,maskrcnn,transformer,nmt,others". Use "others" if you are not sure which tags you need.
# -Dbuckets="oss://xiongmu-sqlflow/?role_arn=acs:ram::1696257213098919:role/paitf&host=cn-zhangjiakou.oss-internal.aliyun-inc.com" -DcheckpointDir="oss://xiongmu-sqlflow/?role_arn=acs:ram::1696257213098919:role/paitf&host=cn-zhangjiakou.oss-internal.aliyun-inc.com"
{{if gt .NumWorkers 1}}
pai_cmd = 'pai -name %s -DjobName=%s -Dtags=%s -Dscript=file://%s -DentryFile=%s -DgpuRequired=\'\' -Dtables=odps://%s/tables/%s -Dcluster=\'{\"ps\":{\"count\":{{.NumPS}}}, \"worker\":{\"count\":{{.NumWorkers}}}}\'' % (
    'tensorflow1120', jobname, 'dnn', tarball, '{{.EntryFile}}', '{{.PAIDatabase}}', '{{.PAITable}}')
{{else}}
pai_cmd = 'pai -name %s -DjobName=%s -Dtags=%s -Dscript=file://%s -DentryFile=%s -DgpuRequired=\'\' -Dtables=odps://%s/tables/%s' % (
    'tensorflow1120', jobname, 'dnn', tarball, '{{.EntryFile}}', '{{.PAIDatabase}}', '{{.PAITable}}')
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
model.save("{{.DataSource}}", "{{.ModelName}}",
           "{{.Save}}",
           "{{.Estimator}}",
           "{{.IsKerasModel}}" == "true",
           feature_column_names,
           feature_metas,
           label_meta,
           model_params,
           feature_columns)
`

const tfPredictTmplText = `
import tensorflow as tf
from sqlflow_submitter.pai import model
from sqlflow_submitter.tensorflow import predict
try:
    tf.enable_eager_execution()
except:
    pass

(save,
 estimator,
 is_keras_model,
 feature_column_names,
 feature_metas,
 label_meta,
 model_params,
 feature_columns) = model.load("{{.DataSource}}", "{{.ModelName}}")

predict.pred(is_keras_model=is_keras_model,
    datasource="{{.DataSource}}",
    estimator=eval(estimator),
    select="""{{.Select}}""",
    result_table="{{.ResultTable}}",
    feature_columns=feature_columns,
    feature_column_names=feature_column_names,
    feature_metas=feature_metas,
    label_meta=label_meta,
    model_params=model_params,
    save=save,
    batch_size=1)
`
