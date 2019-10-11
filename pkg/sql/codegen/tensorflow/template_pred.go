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

const tfPredTemplateText = `
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

from sqlflow_submitter.db import connect, db_generator, buffered_db_writer

# Disable Tensorflow INFO and WARNING
import logging
tf.get_logger().setLevel(logging.ERROR)


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

conn = connect(driver, database, user="{{.User}}", password="{{.Password}}", host="{{.Host}}", port={{.Port}}, auth="{{.Auth}}",session_cfg=session_cfg)

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


{{if .IsKerasModel}}
def eval_input_fn(batch_size):
    feature_types = []
    for name in feature_column_names:
        {{/* NOTE: vector columns like 23,21,3,2,0,0 should use shape None */}}
        if feature_metas[name]["is_sparse"]:
            feature_types.append((tf.int64, tf.int32, tf.int64))
        else:
            feature_types.append(get_dtype(feature_metas[name]["dtype"]))

    gen = db_generator(driver, conn, """{{.PredictionDatasetSQL}}""",
        feature_column_names, "{{.Y.FeatureName}}", feature_metas)
    dataset = tf.data.Dataset.from_generator(gen, (tuple(feature_types), tf.{{.Y.Dtype}}))
    ds_mapper = functools.partial(_parse_sparse_feature, feature_metas=feature_metas)
    dataset = dataset.map(ds_mapper).batch(batch_size)
    return dataset

# NOTE: always use batch_size=1 when predicting to get the pairs of features and predict results
#       to insert into result table.
pred_dataset = eval_input_fn(1)
one_batch = pred_dataset.__iter__().next()
# NOTE: must run predict one batch to initialize parameters
# see: https://www.tensorflow.org/alpha/guide/keras/saving_and_serializing#saving_subclassed_models
classifier.predict_on_batch(one_batch[0])
classifier.load_weights("{{.Save}}")
del pred_dataset
pred_dataset = eval_input_fn(1).make_one_shot_iterator()
buff_rows = []
column_names = feature_column_names[:]
column_names.append("{{.Y.FeatureName}}")
with buffered_db_writer(driver, conn, "{{.TableName}}", column_names, 100) as w:
    while True:
        try:
            features = pred_dataset.get_next()
        except tf.errors.OutOfRangeError:
            break
        result = classifier.predict_on_batch(features[0])
        result = classifier.prepare_prediction_column(result[0])
        row = []
        for idx, name in enumerate(feature_column_names):
            val = features[0][name].numpy()[0]
            row.append(str(val))
        row.append(str(result))
        w.write(row)
del pred_dataset
{{else}}

def fast_input_fn(generator):
    feature_types = []
    for name in feature_column_names:
        if feature_metas[name]["is_sparse"]:
            feature_types.append((tf.int64, tf.int32, tf.int64))
        else:
            feature_types.append(get_dtype(feature_metas[name]["dtype"]))

    def _inner_input_fn():
        dataset = tf.data.Dataset.from_generator(generator, (tuple(feature_types), tf.{{.Y.Dtype}}) )
        ds_mapper = functools.partial(_parse_sparse_feature, feature_metas=feature_metas)
        dataset = dataset.map(ds_mapper).batch(1)
        iterator = dataset.make_one_shot_iterator()
        features = iterator.get_next()
        return features

    return _inner_input_fn

class FastPredict:
    def __init__(self, estimator, input_fn):
        self.estimator = estimator
        self.first_run = True
        self.closed = False
        self.input_fn = input_fn

    def _create_generator(self):
        while not self.closed:
            yield self.next_features[0], self.next_features[1]

    def predict(self, feature_batch):
        self.next_features = feature_batch
        if self.first_run:
            self.batch_size = len(feature_batch)
            self.predictions = self.estimator.predict(
                input_fn=self.input_fn(self._create_generator))
            self.first_run = False
        elif self.batch_size != len(feature_batch):
            raise ValueError("All batches must be of the same size. First-batch:" + str(self.batch_size) + " This-batch:" + str(len(feature_batch)))

        results = []
        for _ in range(self.batch_size):
            results.append(next(self.predictions))
        return results

    def close(self):
        self.closed = True
        try:
            next(self.predictions)
        except Exception as e:
            print("Exception in fast_predict. This is probably OK: %s" % e)

column_names = feature_column_names[:]
column_names.append("{{.Y.FeatureName}}")
pred_gen = db_generator(driver, conn, """{{.PredictionDatasetSQL}}""", feature_column_names, "{{.Y.FeatureName}}", feature_metas)()
fast_predictor = FastPredict(classifier, fast_input_fn)

with buffered_db_writer(driver, conn, "{{.TableName}}", column_names, 100) as w:
    while True:
        try:
            features = pred_gen.__next__()
        except StopIteration:
            break
        result = fast_predictor.predict(features)
        row = []
        for idx, _ in enumerate(feature_column_names):
            val = features[0][idx]
            row.append(str(val))
        if "class_ids" in list(result)[0]:
            row.append(str(list(result)[0]["class_ids"][0]))
        else:
            # regression predictions
            row.append(str(list(result)[0]["predictions"][0]))
        w.write(row)

{{end}}

print("Done predicting. Predict table : {{.TableName}}")
`
