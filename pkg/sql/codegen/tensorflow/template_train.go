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

import "sqlflow.org/sqlflow/pkg/sql/codegen"

type trainFiller struct {
	DataSource        string
	TrainSelect       string
	ValidationSelect  string
	FieldMetas        []*codegen.FieldMeta
	FeatureColumnCode []string
	Y                 codegen.FeatureColumn
	modelParams       map[string]string
	trainParams       map[string]interface{}
}

const tfTrainTemplateText = `
import os
# Disable Tensorflow INFO and WARNING logs
os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'

import sys, json
import tensorflow as tf
import functools
try:
    import sqlflow_models
except:
    pass

from sqlflow_submitter.db import connect_with_data_source, db_generator

# Disable Tensorflow INFO and WARNING
import logging
tf.get_logger().setLevel(logging.ERROR)

BATCHSIZE = {{.BatchSize}}
EPOCHS = {{.Epochs}}
VERBOSE = {{.Verbose}}

session_cfg = {}
{{ range $k, $v := .Session }}
session_cfg["{{$k}}"] = "{{$v}}"
{{end}}

conn = connect_with_data_source("{{.DataSource}}")

feature_column_names = [{{range .X}}
"{{.FeatureName}}",
{{end}}]


classifier = {{.EstimatorCode}}(
    {{.FeatureColumnParmas}},
    {{.AttrParams}},
    {{if .IsKerasModel}}
)
    {{else}}
    model_dir = "{{.Save}}")
    {{end}}

{{/* Convert go side featureSpec to python dict for input_fn */}}
feature_metas = dict()
{{ range $value := .X }}
feature_metas["{{$value.FeatureName}}"] = {
    "feature_name": "{{$value.FeatureName}}",
    "dtype": "{{$value.Dtype}}",
    "delimiter": "{{$value.Delimiter}}",
    "shape": {{$value.InputShape}},
    "is_sparse": "{{$value.IsSparse}}" == "true"
}
{{end}}

def get_dtype(type_str):
    if type_str == "float32":
        return tf.float32
    elif type_str == "int64":
        return tf.int64
    else:
        raise TypeError("not supported dtype: %s" % type_str)

def _parse_sparse_feature(features, label, feature_metas):
    features_dict = dict()
    for idx, col in enumerate(features):
        name = feature_column_names[idx]
        if feature_metas[name]["is_sparse"]:
            i, v, s = col
            features_dict[name] = tf.SparseTensor(indices=i, values=v, dense_shape=s)
        else:
            features_dict[name] = col
    return features_dict, label


def input_fn(datasetStr):
    feature_types = []
    for name in feature_column_names:
        {{/* NOTE: vector columns like 23,21,3,2,0,0 should use shape None */}}
        if feature_metas[name]["is_sparse"]:
            feature_types.append((tf.int64, tf.int32, tf.int64))
        else:
            feature_types.append(get_dtype(feature_metas[name]["dtype"]))

    gen = db_generator(conn.driver, conn, datasetStr, feature_column_names, "{{.Y.FeatureName}}", feature_metas)
    dataset = tf.data.Dataset.from_generator(gen, (tuple(feature_types), tf.{{.Y.Dtype}}))
    ds_mapper = functools.partial(_parse_sparse_feature, feature_metas=feature_metas)
    return dataset.map(ds_mapper)

def train_input_fn(batch_size):
    dataset = input_fn("""{{.TrainingDatasetSQL}}""")
    # TODO(typhoonzero): add prefetch, cache if needed.
    dataset = dataset.shuffle(1000).batch(batch_size)
    {{if not .IsKerasModel}}
    {{/* estimater.train have no argument epochs, so add in dataset here */}}
    dataset = dataset.repeat(EPOCHS if EPOCHS else 1)
    {{end}}
    return dataset

def validate_input_fn(batch_size):
    dataset = input_fn("""{{.ValidationDatasetSQL}}""")
    return dataset.batch(batch_size)

{{if .IsKerasModel}}
classifier.compile(optimizer=classifier.default_optimizer(),
    loss=classifier.default_loss(),
    metrics=["accuracy"])
if hasattr(classifier, 'sqlflow_train_loop'):
    classifier.sqlflow_train_loop(train_input_fn(BATCHSIZE))
else:
    classifier.fit(train_input_fn(BATCHSIZE),
        epochs=EPOCHS if EPOCHS else classifier.default_training_epochs(),
        verbose=VERBOSE)
classifier.save_weights("{{.Save}}", save_format="h5")
if "{{.Y.FeatureName}}" != "":
    eval_result = classifier.evaluate(validate_input_fn(BATCHSIZE), verbose=VERBOSE)
    print("Training set accuracy: {accuracy:0.5f}".format(**{"accuracy": eval_result[1]}))
{{else}}
classifier.train(input_fn=lambda:train_input_fn(BATCHSIZE))
eval_result = classifier.evaluate(input_fn=lambda:validate_input_fn(BATCHSIZE))
print("Evaluation result:", eval_result)
{{end}}

print("Done training")
`
