package sql

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/go-sql-driver/mysql"
	"sqlflow.org/gohive"
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

	if ctype == "FLOAT" || ctype == "INT" || ctype == "DOUBLE" {
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
			SelfDefined: isSelfDefinedModel}}
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
		r.TableName = strings.Join(strings.Split(pr.into, ".")[:2], ".")
	}

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

{{if eq .Driver "mysql"}}
from mysql.connector import connect
{{else if eq .Driver "sqlite3"}}
from sqlite3 import connect
{{else if eq .Driver "hive"}}
from impala.dbapi import connect 
{{end}}

# Disable Tensorflow INFO and WARNING
import logging
tf.get_logger().setLevel(logging.ERROR)
` +
	// TODO(tonyyang-svail): remove hard coded BATCHSIZE, STEP
	// TODO(typhoonzero): get NUM_BUCKETS, EMBEDDING_WIDTH from Extended SQL statements
	`
BATCHSIZE = 1
STEP = 1000
NUM_BUCKETS=160000
EMBEDDING_WIDTH=128

{{if eq .Driver "mysql"}}
db = connect(user="{{.User}}",
            passwd="{{.Password}}",
            {{if ne .Database ""}}database="{{.Database}}",{{end}}
            host="{{.Host}}",
            port={{.Port}})
{{else if eq .Driver "sqlite3"}}
db = connect({{.Database}})
{{else if eq .Driver "hive"}}
db = connect(user="{{.User}}",
            password="{{.Password}}",
            {{if ne .Database ""}}database="{{.Database}}",{{end}}
            host="{{.Host}}",
            port={{.Port}})
{{else}}
raise ValueError("unrecognized database driver: {{.Driver}}")
{{end}}

cursor = db.cursor()
cursor.execute("""{{.StandardSelect}}""")
{{if eq .Driver "hive"}}
field_names = [i[0][i[0].find('.')+1:] for i in cursor.description]
{{else}}
field_names = [i[0] for i in cursor.description]
{{end}}
columns = list(map(list, zip(*cursor.fetchall())))

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

{{if .Train}}
Y = columns[field_names.index("{{.Y.Name}}")]
{{- end}}

classifier = {{.Estimator}}(
	feature_columns=feature_columns,{{if .SelfDefined}}{{range $key, $value := .Attrs}}
	{{$key}} = {{$value}},{{end}})
	{{else}}{{range $key, $value := .Attrs}}
	{{$key}} = {{$value}},{{end}}
	model_dir = "{{.Save}}")
	{{end}}
{{if .SelfDefined}}
classifier.compile(optimizer=classifier.default_optimizer(),
	loss=classifier.default_loss(),
	metrics=["accuracy"])
{{end}}

{{if .Train}}
def train_input_fn(features, labels, batch_size):
    dataset = tf.data.Dataset.from_tensor_slices((dict(features), labels))
    dataset = dataset.shuffle(1000).repeat().batch(batch_size)
    return dataset

{{if .SelfDefined}}
classifier.fit(train_input_fn(X, Y, BATCHSIZE),
	epochs=classifier.default_training_epochs(),
	steps_per_epoch=STEP, verbose=0)
{{else}}
classifier.train(
    input_fn=lambda:train_input_fn(X, Y, BATCHSIZE),
    steps=STEP)
{{end}}

def eval_input_fn(features, labels, batch_size):
    dataset = tf.data.Dataset.from_tensor_slices((dict(features), labels))
    dataset = dataset.batch(batch_size)
    return dataset

{{if .SelfDefined}}
eval_result = classifier.evaluate(eval_input_fn(X, Y, BATCHSIZE), verbose=0)
print("Training set accuracy: {accuracy:0.5f}".format(**{"accuracy": eval_result[1]}))
{{else}}
eval_result = classifier.evaluate(
    input_fn=lambda:eval_input_fn(X, Y, BATCHSIZE), steps=STEP)
print("Training set accuracy: {accuracy:0.5f}".format(**eval_result))
{{end}}
print("Done training")
{{- else}}
def eval_input_fn(features, batch_size):
    dataset = tf.data.Dataset.from_tensor_slices(dict(features))
    dataset = dataset.batch(batch_size)
    return dataset

{{if .SelfDefined}}
predictions = classifier.predict(eval_input_fn(X, BATCHSIZE))
X["{{.Y.Name}}"] = [p[classifier.prepare_prediction_column(p)] for p in predictions]
{{else}}
predictions = classifier.predict(input_fn=lambda:eval_input_fn(X, BATCHSIZE))
X["{{.Y.Name}}"] = [p['class_ids'][0] for p in predictions]
{{end}}

def insert(table_name, X, db):
    length = [len(X[key]) for key in X]
    assert len(set(length)) == 1, "All the fields should have the same length"

    field_names = [key for key in X]
    {{if eq .Driver "hive"}}
    sql = "INSERT INTO TABLE {} ({}) VALUES ({})".format(table_name,
        ",".join(field_names), ",".join(["%s" for _ in field_names]))
    {{else}}
    sql = "INSERT INTO {} ({}) VALUES ({})".format(table_name,
        ",".join(field_names), ",".join(["%s" for _ in field_names]))
    {{end}}

    val = []
    for i in range(length[0]):
        val.append(tuple([str(X[f][i]) for f in field_names]))

    cursor = db.cursor()
    cursor.executemany(sql, val)
    db.commit()

insert("{{.TableName}}", X, db)

print("Done predicting. Predict table : {{.TableName}}")
{{- end}}
`

var codegenTemplate = template.Must(template.New("codegen").Parse(codegenTemplateText))
