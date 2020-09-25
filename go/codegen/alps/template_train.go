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

package alps

import "sqlflow.org/sqlflow/go/ir"

type trainFiller struct {
	DataSource        string
	TrainSelect       string
	ValidationSelect  string
	Estimator         string
	FieldDescs        map[string][]*ir.FieldDesc
	FeatureColumnCode string
	Y                 *ir.FieldDesc
	ModelParams       map[string]interface{}
	TrainParams       map[string]interface{}
	ValidationParams  map[string]interface{}
	Save              string
	TmpTrainTable     string
	TmpValidateTable  string
}

var templateTrain = `import copy
import os
import shutil

import tensorflow as tf
from alps.framework.column.column import (DenseColumn, GroupedSparseColumn,
                                          SparseColumn)
from alps.framework.engine import LocalEngine
from alps.framework.experiment import EstimatorBuilder
from alps.io.base import OdpsConf
from runtime.model import db
from runtime.alps.train import train
from runtime.tensorflow.get_tf_version import tf_is_version2

feature_column_names = [{{range $target, $desclist := .FieldDescs}}{{range $desclist}}
"{{.Name}}",
{{end}}{{end}}]

feature_metas = dict()
{{ range $target, $desclist := .FieldDescs }}
{{ range $value := $desclist }}
feature_metas["{{$value.Name}}"] = {
    "feature_name": "{{$value.Name}}",
    "dtype": "{{$value.DType | DTypeToString}}",
    "delimiter": "{{$value.Delimiter}}",
    "format": "{{$value.Format}}",
    "shape": {{$value.Shape | intArrayToJSONString}},
    "is_sparse": "{{$value.IsSparse}}" == "true",
    "dtype_weight": "{{$value.DTypeWeight | DTypeToString}}",
    "delimiter_kv": "{{$value.DelimiterKV}}"
}
{{end}}
{{end}}

label_meta = {
    "feature_name": "{{.Y.Name}}",
    "dtype": "{{.Y.DType | DTypeToString}}",
    "delimiter": "{{.Y.Delimiter}}",
    "shape": {{.Y.Shape | intArrayToJSONString}},
    "is_sparse": "{{.Y.IsSparse}}" == "true"
}

model_params=dict()
{{range $k, $v := .ModelParams}}
model_params["{{$k}}"]={{$v | attrToPythonValue}}
{{end}}

# Construct optimizer objects to pass to model initializer.
# The original model_params is serializable (do not have tf.xxx objects).
model_params_constructed = copy.deepcopy(model_params)
for optimizer_arg in ["optimizer", "dnn_optimizer", "linear_optimizer"]:
    if optimizer_arg in model_params_constructed:
        model_params_constructed[optimizer_arg] = eval(model_params_constructed[optimizer_arg])

if "loss" in model_params_constructed:
    model_params_constructed["loss"] = eval(model_params_constructed["loss"])


class SQLFlowEstimatorBuilder(EstimatorBuilder):
    def _build(self, experiment, run_config):
        feature_columns_map = {{.FeatureColumnCode}}
        if feature_columns_map.get("feature_columns"):
            feature_columns = feature_columns_map["feature_columns"]
        else:
            raise ValueError("Not supported feature column map")
        model_params_constructed["feature_columns"] = feature_columns
        return tf.estimator.{{.Estimator}}(config=run_config,
                                           **model_params_constructed)


if __name__ == "__main__":
    if tf_is_version2():
        raise ValueError("ALPS must run with TensorFlow == 1.15.x")

    driver, dsn = "{{.DataSource}}".split("://")
    user, passwd, endpoint, odps_project = db.parseMaxComputeDSN(dsn)
    odps_conf = OdpsConf(
        accessid=user,
        accesskey=passwd,
        # endpoint should looks like: "https://service.cn.maxcompute.aliyun.com/api"
        endpoint=endpoint,
        project=odps_project)

    features = []
    for col_name in feature_column_names:
        # NOTE: add sparse columns like: SparseColumn(name="deep_id", shape=[15033], dtype="int")
        if feature_metas[col_name]["is_sparse"]:
            features.append(SparseColumn(name=feature_metas[col_name]["feature_name"],
                                         shape=feature_metas[col_name]["shape"],
                                         dtype=feature_metas[col_name]["dtype"],
                                         separator=feature_metas[col_name]["separator"]))
        else:
            features.append(DenseColumn(name=feature_metas[col_name]["feature_name"],
                                        shape=feature_metas[col_name]["shape"],
                                        dtype=feature_metas[col_name]["dtype"]))
    labels = DenseColumn(name=label_meta["feature_name"],
                         shape=label_meta["shape"],
                         dtype=label_meta["dtype"])

    try:
        os.mkdir("scratch")
    except FileExistsError:
        pass

    train_max_steps = {{index .TrainParams "max_steps" | attrToPythonValue}}
    train_max_steps = None if train_max_steps == 0 else train_max_steps

    # TODO(typhoonzero): support pass feature_map_table from WITH attributes.
    # TODO(typhoonzero): pass actual use_id.
    # TODO(typhoonzero): pass engine config to submit jobs to the cluster.
    train(SQLFlowEstimatorBuilder(),
          odps_conf=odps_conf,
          project=odps_project,
          train_table="{{.TmpTrainTable}}",
          eval_table="{{.TmpValidateTable}}",
          features=features,
          labels=labels,
          feature_map_table="",
          feature_map_partition="",
          epochs=1,
          batch_size=2,
          shuffle=False,
          shuffle_bufsize=128,
          cache_file="",
          max_steps=train_max_steps,
          eval_steps={{index .ValidationParams "steps" | attrToPythonValue}},
          eval_batch_size=1,
          eval_start_delay={{index .ValidationParams "start_delay_secs" | attrToPythonValue}},
          eval_throttle={{index .ValidationParams "throttle_secs" | attrToPythonValue}},
          drop_remainder=True,
          export_path="./scratch/model",
          scratch_dir="./scratch",
          user_id="",
          engine_config={"name": "LocalEngine"},
          exit_on_submit=False)
    shutil.rmtree("scratch")
`
