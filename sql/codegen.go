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
	Estimator   string
	Attrs       map[string]string
	Save        string
	SelfDefined bool
}

type filler struct {
	Train          bool
	Driver         string
	StandardSelect string
	modelConfig
	X         []columnType
	Y         columnType
	TableName string
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
		return &columnType{ident, "categorical_column_with_identity"}, nil
	}
	return nil, fmt.Errorf("unsupported type %s of field %s", ctype, ident)
}

// parseModelURI returns isSelfDefinedModel, modelClassString
func parseModelURI(modelString string) (bool, string) {
	if strings.HasPrefix(modelString, "sqlflow_models.") {
		return true, modelString
	}
	return false, fmt.Sprintf("tf.estimator.%s", modelString)
}

// TODO(weiguo): fts -> pointer
func newFiller(pr *extendedSelect, fts fieldTypes, db *DB) (*filler, error) {
	isSelfDefinedModel, modelClassString := parseModelURI(pr.estimator)
	r := &filler{
		Train:          pr.train,
		StandardSelect: pr.standardSelect.String(),
		modelConfig: modelConfig{
			Estimator:   modelClassString,
			Attrs:       make(map[string]string),
			Save:        pr.save,
			SelfDefined: isSelfDefinedModel,
		},
	}
	for k, v := range pr.attrs {
		r.Attrs[k] = v.String()
	}

	for _, c := range pr.columns {
		cf, e := translateColumnToFeature(&fts, db.driverName, c.val)
		if e != nil {
			return nil, e
		}
		r.X = append(r.X, *cf)
	}

	cf, e := translateColumnToFeature(&fts, db.driverName, pr.label)
	if e != nil {
		return nil, e
	}
	r.Y = *cf

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

func genTF(w io.Writer, pr *extendedSelect, fts fieldTypes, db *DB) error {
	r, e := newFiller(pr, fts, db)
	if e != nil {
		return e
	}
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
try:
	import sqlflow_models
except:
	pass

from sqlflow_submitter.db import connect, insert_values, execute, db_generator

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

train_args = dict()
{{range $key, $value := .Attrs}}
{{if eq $key "BATCHSIZE"}}
BATCHSIZE = {{$value}}
{{else if eq $key "EPOCHS"}}
EPOCHS = {{$value}}
{{else}}
train_args["{{$key}}"] = {{$value}}
{{end}}
{{end}}

driver="{{.Driver}}"
{{if ne .Database ""}}
database="{{.Database}}"
{{else}}
database=None
{{end}}

conn = connect(driver, database, user="{{.User}}", password="{{.Password}}", host="{{.Host}}", port={{.Port}})

feature_columns = []
column_name_to_type = dict()
{{range .X}}
column_name_to_type["{{.Name}}"] = "{{.Type}}"
{{if eq .Type "categorical_column_with_identity"}}
feature_columns.append(tf.feature_column.embedding_column(
	tf.feature_column.categorical_column_with_identity(
	key="{{.Name}}",
	num_buckets=NUM_BUCKETS),
dimension=EMBEDDING_WIDTH))
{{else}}
feature_columns.append(tf.feature_column.{{.Type}}(key="{{.Name}}"))
{{end}}
{{end}}

feature_column_names = [{{range .X}}"{{.Name}}",
{{end}}]


classifier = {{.Estimator}}(
	feature_columns=feature_columns,
	**train_args,
	{{if .SelfDefined}}
)
	{{else}}
	model_dir = "{{.Save}}")
	{{end}}

{{if .Train}}
def input_fn(batch_size, is_train=True):
	feature_types = dict()
	feature_shapes = dict()
	for name in feature_column_names:
		if column_name_to_type[name] == "categorical_column_with_identity":
			feature_types[name] = tf.int64
			feature_shapes[name] = tf.TensorShape([None])
		else:
			feature_types[name] = tf.float32
			feature_shapes[name] = tf.TensorShape([])

	gen = db_generator(driver, conn, """{{.StandardSelect}}""",
		feature_column_names, "{{.Y.Name}}", column_name_to_type)
	dataset = tf.data.Dataset.from_generator(gen, (feature_types, tf.int64), (feature_shapes, tf.TensorShape([1])))
	if is_train:
		# TODO(typhoonzero): add prefetch, cache if needed.
		dataset = dataset.shuffle(1000).batch(batch_size)
		{{if not .SelfDefined}}
		{{/* estimater.train have no argument epochs, so add in dataset here */}}
		dataset = dataset.repeat(EPOCHS if EPOCHS else 1)
		{{end}}
	else:
		dataset = dataset.batch(batch_size)
	return dataset

{{if .SelfDefined}}
classifier.compile(optimizer=classifier.default_optimizer(),
	loss=classifier.default_loss(),
	metrics=["accuracy"])
classifier.fit(input_fn(BATCHSIZE, is_train=True),
	epochs=EPOCHS if EPOCHS else classifier.default_training_epochs(),
	verbose=0)
classifier.save_weights("{{.Save}}", save_format="h5")
{{else}}
classifier.train(
	input_fn=lambda:input_fn(BATCHSIZE, is_train=True))
{{end}}

{{if .SelfDefined}}
eval_result = classifier.evaluate(input_fn(BATCHSIZE, is_train=False), verbose=0)
print("Training set accuracy: {accuracy:0.5f}".format(**{"accuracy": eval_result[1]}))
{{else}}
eval_result = classifier.evaluate(
	input_fn=lambda:input_fn(BATCHSIZE, is_train=False))
print(eval_result)
print("Training set accuracy: {accuracy:0.5f}".format(**eval_result))
{{end}}
print("Done training")
{{- else}}

def eval_input_fn(features, batch_size):
	dataset = tf.data.Dataset.from_tensor_slices(dict(features))
	return dataset.batch(batch_size)

field_names, columns = execute(driver, conn, """{{.StandardSelect}}""")
X = {}
for name in feature_column_names:
	if column_name_to_type[name] == "categorical_column_with_identity":
		rows = columns[field_names.index(name)]
		# convert to int tensors
		tensor_rows = []
		for row in rows:
			tensor_rows.append(np.array([int(v) for v in row.split(",")]))
		X[name] = np.array(tensor_rows)
	else:
		X = {name: columns[field_names.index(name)] for name in feature_column_names}

{{if .SelfDefined}}
pred_dataset = eval_input_fn(X, BATCHSIZE)
one_batch = pred_dataset.__iter__().next()
# NOTE: must run predict one batch to initialize parameters
# see: https://www.tensorflow.org/alpha/guide/keras/saving_and_serializing#saving_subclassed_models
classifier.predict_on_batch(one_batch)
classifier.load_weights("{{.Save}}")
del pred_dataset
pred_dataset = eval_input_fn(X, BATCHSIZE)
predictions = classifier.predict(pred_dataset)
X["{{.Y.Name}}"] = [classifier.prepare_prediction_column(p) for p in predictions]
{{else}}
predictions = classifier.predict(input_fn=lambda:eval_input_fn(X, BATCHSIZE))
X["{{.Y.Name}}"] = [p['class_ids'][0] for p in predictions]
{{end}}

def insert(table_name, X):
    length = [len(X[key]) for key in X]
    assert len(set(length)) == 1, "All the fields should have the same length"

    field_names = [key for key in X]

    val = []
    for i in range(length[0]):
        val.append(tuple([str(X[f][i]) for f in field_names]))

    insert_values(driver, conn, "{{.TableName}}", field_names, val)

insert("{{.TableName}}", X)

print("Done predicting. Predict table : {{.TableName}}")
{{- end}}
`

var codegenTemplate = template.Must(template.New("codegen").Parse(codegenTemplateText))
