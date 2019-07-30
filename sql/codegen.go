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

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/go-sql-driver/mysql"
	"sqlflow.org/gohive"
	"sqlflow.org/gomaxcompute"
)

type columnType struct {
	Name string
	Type string
}

type connectionConfig struct {
	User     string
	Password string
	Host     string
	Port     string
	Database string
}

type modelConfig struct {
	Estimator    string
	Attrs        map[string]string
	Save         string
	IsKerasModel bool
}

type featureMeta struct {
	FeatureName string
	Dtype       string
	Delimiter   string
	InputShape  string
	IsSparse    bool
}

type filler struct {
	IsTrain        bool
	Driver         string
	StandardSelect string
	X              []*featureMeta
	// key: for target (e.g. deep-wide model), value: list of generated code for current target
	FeatureColumnsCode map[string][]string
	Y                  *featureMeta
	TableName          string
	modelConfig
	connectionConfig
}

func translateColumnToFeature(fts *fieldTypes, driverName, ident string) (*columnType, error) {
	ct, ok := fts.get(ident)
	if !ok {
		return nil, fmt.Errorf("genTF: Cannot find type of field %s", ident)
	}
	ctype, e := universalizeColumnType(driverName, ct)
	if e != nil {
		return nil, e
	}
	ctype = strings.ToUpper(ctype)

	if ctype == "FLOAT" || ctype == "INT" || ctype == "DOUBLE" || ctype == "BIGINT" {
		return &columnType{ident, "numeric_column"}, nil
	} else if ctype == "TEXT" || ctype == "VARCHAR" {
		// FIXME(typhoonzero): only support preprocessed string of int vector
		// like: "231,291,0,0,9", to support arbitrary string, we need to provide
		// additional information like how to parse.
		// TODO(typhoonzero): need to support categorical_column_with_vocabulary_list
		// which read vocabulary from DB.
		// return &columnType{ident, "categorical_column_with_identity"}, nil
		return &columnType{ident, "categorical_column_with_identity"}, nil
	}
	return nil, fmt.Errorf("unsupported type %s of field %s", ctype, ident)
}

// parseModelURI returns isKerasModel, modelClassString
func parseModelURI(modelString string) (bool, string) {
	if strings.HasPrefix(modelString, "sqlflow_models.") {
		return true, modelString
	}
	return false, fmt.Sprintf("tf.estimator.%s", modelString)
}

func newFiller(pr *extendedSelect, ds *trainAndValDataset, fts fieldTypes, db *DB) (*filler, error) {
	// TODO(weiguo): modify filler struct to carry trainingDatase in the next PR
	isKerasModel, modelClassString := parseModelURI(pr.estimator)
	r := &filler{
		IsTrain:        pr.train,
		StandardSelect: pr.standardSelect.String(),
		modelConfig: modelConfig{
			Estimator:    modelClassString,
			Attrs:        make(map[string]string),
			Save:         pr.save,
			IsKerasModel: isKerasModel,
		},
	}
	for k, v := range pr.attrs {
		r.Attrs[k] = v.String()
	}

	for target, columns := range pr.columns {
		feaCols, colSpecs, err := resolveTrainColumns(&columns)
		if err != nil {
			return nil, err
		}
		if len(colSpecs) != 0 {
			return nil, fmt.Errorf("newFiller doesn't support DENSE/SPARSE")
		}
		r.FeatureColumnsCode = make(map[string][]string)
		for _, col := range feaCols {
			feaColCode, e := col.GenerateCode()
			if e != nil {
				return nil, e
			}
			// FIXME(typhoonzero): Use Heuristic rules to determine whether a column should be transformed to a
			// tf.SparseTensor. Currently the rules are:
			// if column have delimiter and it's not a sequence_catigorical_column, we'll treat it as a sparse column
			// else, use dense column.
			isSparse := false
			var isEmb bool
			_, ok := col.(*sequenceCategoryIDColumn)
			if !ok {
				_, isEmb = col.(*embeddingColumn)
				if isEmb {
					_, ok = col.(*embeddingColumn).CategoryColumn.(*sequenceCategoryIDColumn)
				}
			}
			if !ok && col.GetDelimiter() != "" {
				if _, ok := col.(*numericColumn); !ok {
					isSparse = true
				}
			}
			fm := &featureMeta{
				FeatureName: col.GetKey(),
				Dtype:       col.GetDtype(),
				Delimiter:   col.GetDelimiter(),
				InputShape:  col.GetInputShape(),
				IsSparse:    isSparse,
			}
			r.X = append(r.X, fm)
			r.FeatureColumnsCode[target] = append(
				r.FeatureColumnsCode[target],
				feaColCode)
		}
	}

	// for k, v := range fts {
	// 	fmt.Printf("field: %s, types: %v\n", k, v)
	// }
	// Default use int32 label dtype
	labelDtype := "int32"
	if val, ok := fts[pr.label]; ok {
		for _, v := range val {
			if v == "FLOAT" {
				labelDtype = "float32"
			} else if v == "DOUBLE" {
				labelDtype = "float64"
			} else if v == "INT" || v == "INT_TYPE" {
				labelDtype = "int32"
			} else if v == "BIGINT" {
				labelDtype = "int64"
			} else {
				log.Fatalf("Unsupported label data type: %s", v)
			}
			// TODO(typhoonzero): get the dtype from first appeared table name.
			// fix this if we have multiple tables in select statement.
			break
		}
	}
	r.Y = &featureMeta{
		FeatureName: pr.label,
		Dtype:       labelDtype,
		Delimiter:   ",",
		InputShape:  "[1]",
		IsSparse:    false,
	}

	var e error
	if !pr.train {
		if r.TableName, _, e = parseTableColumn(pr.into); e != nil {
			return nil, e
		}
	}

	return fillDatabaseInfo(r, db)
}

func fillDatabaseInfo(r *filler, db *DB) (*filler, error) {
	r.Driver = db.driverName
	switch db.driverName {
	case "mysql":
		cfg, err := mysql.ParseDSN(db.dataSourceName)
		if err != nil {
			return nil, err
		}
		sa := strings.Split(cfg.Addr, ":")
		r.Host, r.Port, r.Database = sa[0], sa[1], cfg.DBName
		r.User, r.Password = cfg.User, cfg.Passwd
	case "sqlite3":
		r.Database = db.dataSourceName
	case "hive":
		cfg, err := gohive.ParseDSN(db.dataSourceName)
		if err != nil {
			return nil, err
		}
		sa := strings.Split(cfg.Addr, ":")
		r.Host, r.Port, r.Database = sa[0], sa[1], cfg.DBName
		r.User, r.Password = cfg.User, cfg.Passwd
		// remove the last ';' which leads to a ParseException
		r.StandardSelect = removeLastSemicolon(r.StandardSelect)
	case "maxcompute":
		cfg, err := gomaxcompute.ParseDSN(db.dataSourceName)
		if err != nil {
			return nil, err
		}
		// setting r.Port=0 just makes connect() happy
		r.Host, r.Port, r.Database = cfg.Endpoint, "0", cfg.Project
		r.User, r.Password = cfg.AccessID, cfg.AccessKey
	default:
		return nil, fmt.Errorf("sqlfow currently doesn't support DB %v", db.driverName)
	}
	return r, nil
}

func removeLastSemicolon(s string) string {
	n := len(s)
	if n > 0 && s[n-1] == ';' {
		return s[0 : n-1]
	}
	return s
}

func genTF(w io.Writer, pr *extendedSelect, ds *trainAndValDataset, fts fieldTypes, db *DB) error {
	r, e := newFiller(pr, ds, fts, db)
	if e != nil {
		return e
	}
	// TODO(weiguo): fix codegen to carry trainingDatase in the next PR
	if e = codegenTemplate.Execute(w, r); e != nil {
		return fmt.Errorf("genTF: failed executing template: %v", e)
	}
	return nil
}

const codegenTemplateText = `
import os
# Disable Tensorflow INFO and WARNING logs
os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'

import sys, json
import tensorflow as tf
import numpy as np
import functools
try:
    import sqlflow_models
except:
    pass

from sqlflow_submitter.db import connect, insert_values, db_generator

# Disable Tensorflow INFO and WARNING
import logging
tf.get_logger().setLevel(logging.ERROR)

` +
	// TODO(typhoonzero): get NUM_BUCKETS, EMBEDDING_WIDTH from Extended SQL statements in
	// COLUMN sub clause
	`
BATCHSIZE = 1
EPOCHS = None
NUM_BUCKETS=160000
EMBEDDING_WIDTH=128
VERBOSE = 0

train_args = dict()
{{range $key, $value := .Attrs}}
{{if eq $key "BATCHSIZE"}}
BATCHSIZE = {{$value}}
{{else if eq $key "EPOCHS"}}
EPOCHS = {{$value}}
{{else if eq $key "VERBOSE"}}
VERBOSE = int({{$value}})
{{else}}
train_args["{{$key}}"] = {{$value}}
{{end}}
{{end}}

driver="{{.Driver}}"
{{if ne .Database ""}}
database="{{.Database}}"
{{else}}
database=""
{{end}}

conn = connect(driver, database, user="{{.User}}", password="{{.Password}}", host="{{.Host}}", port={{.Port}})

{{$iskeras := .IsKerasModel}}

feature_columns = dict()
{{ range $target, $colsCode := .FeatureColumnsCode }}
feature_columns["{{$target}}"] = []
{{ range $col := $colsCode }}
feature_columns["{{$target}}"].append({{$col}})
{{ end }}
{{ end }}


feature_column_names = [{{range .X}}
"{{.FeatureName}}",
{{end}}]


classifier = {{.Estimator}}(
    **feature_columns,
    **train_args,
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


{{if .IsTrain}}
def input_fn(batch_size, is_train=True):
    feature_types = []
    for name in feature_column_names:
        {{/* NOTE: vector columns like 23,21,3,2,0,0 should use shape None */}}
        if feature_metas[name]["is_sparse"]:
            feature_types.append((tf.int64, tf.int32, tf.int64))
        else:
            feature_types.append(get_dtype(feature_metas[name]["dtype"]))

    gen = db_generator(driver, conn, """{{.StandardSelect}}""",
        feature_column_names, "{{.Y.FeatureName}}", feature_metas)
    dataset = tf.data.Dataset.from_generator(gen, (tuple(feature_types), tf.{{.Y.Dtype}}))
    ds_mapper = functools.partial(_parse_sparse_feature, feature_metas=feature_metas)
    dataset = dataset.map(ds_mapper)
    if is_train:
        # TODO(typhoonzero): add prefetch, cache if needed.
        dataset = dataset.shuffle(1000).batch(batch_size)
        {{if not .IsKerasModel}}
        {{/* estimater.train have no argument epochs, so add in dataset here */}}
        dataset = dataset.repeat(EPOCHS if EPOCHS else 1)
        {{end}}
    else:
        dataset = dataset.batch(batch_size)
    return dataset

{{if .IsKerasModel}}
classifier.compile(optimizer=classifier.default_optimizer(),
    loss=classifier.default_loss(),
    metrics=["accuracy"])
classifier.fit(input_fn(BATCHSIZE, is_train=True),
    epochs=EPOCHS if EPOCHS else classifier.default_training_epochs(),
    verbose=VERBOSE)
classifier.save_weights("{{.Save}}", save_format="h5")
{{else}}
classifier.train(
    input_fn=lambda:input_fn(BATCHSIZE, is_train=True))
{{end}}

{{if .IsKerasModel}}
eval_result = classifier.evaluate(input_fn(BATCHSIZE, is_train=False), verbose=VERBOSE)
print("Training set accuracy: {accuracy:0.5f}".format(**{"accuracy": eval_result[1]}))
{{else}}
eval_result = classifier.evaluate(
    input_fn=lambda:input_fn(BATCHSIZE, is_train=False))
print("Evaluation result:", eval_result)
{{end}}
print("Done training")
{{- else}}

{{if .IsKerasModel}}
def eval_input_fn(batch_size):
    feature_types = []
    for name in feature_column_names:
        {{/* NOTE: vector columns like 23,21,3,2,0,0 should use shape None */}}
        if feature_metas[name]["is_sparse"]:
            feature_types.append((tf.int64, tf.int32, tf.int64))
        else:
            feature_types.append(get_dtype(feature_metas[name]["dtype"]))

    gen = db_generator(driver, conn, """{{.StandardSelect}}""",
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
    buff_rows.append(row)
    if len(buff_rows) > 100:
        insert_values(driver, conn, "{{.TableName}}", column_names, buff_rows)
        buff_rows.clear()

if len(buff_rows) > 0:
    insert_values(driver, conn, "{{.TableName}}", column_names, buff_rows)
    buff_rows.clear()
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
pred_gen = db_generator(driver, conn, """{{.StandardSelect}}""", feature_column_names, "{{.Y.FeatureName}}", feature_metas)()
fast_predictor = FastPredict(classifier, fast_input_fn)
buff_rows = []
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
    buff_rows.append(row)
    if len(buff_rows) > 100:
        insert_values(driver, conn, "{{.TableName}}", column_names, buff_rows)
        buff_rows.clear()

if len(buff_rows) > 0:
    insert_values(driver, conn, "{{.TableName}}", column_names, buff_rows)
    buff_rows.clear()
{{end}}


print("Done predicting. Predict table : {{.TableName}}")
{{- end}}
`

var codegenTemplate = template.Must(template.New("codegen").Parse(codegenTemplateText))
