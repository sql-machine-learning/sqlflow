package sql

import (
	"text/template"
)

const codegen_template_text = `
import tensorflow as tf
import sys, json, os
import mysql.connector

WORKSPACE = "/tmp/"
SQL_PARSING_RESULT_FILE = "sqlflow.json"

# TODO(tonyyang-svail): Add make sql recognize the following
BATCHSIZE = 1
STEP = 1000
USER = "root"
PASSWORD = "root"
HOST = "localhost"
DATABASE = "yang"

db = mysql.connector.connect(user=USER, passwd=PASSWORD, host=HOST, database=DATABASE)
cursor = db.cursor()
cursor.execute("""{{.StandardSelect}}""")
field_names = [i[0] for i in cursor.description]
columns = map(list, zip(*cursor.fetchall()))

feature_columns = [tf.feature_column.numeric_column(key=key) for key in field_names[:-1]]
X = {field_names[i]: columns[i] for i in range(len(field_names) - 1)}
Y = columns[-1]

{{if .Train}}
classifier = tf.estimator.{{.TrainClause.Estimator}}(
    feature_columns=feature_columns,
    hidden_units={{index .TrainClause.Attrs "hidden_units"}},
    n_classes={{index .TrainClause.Attrs "n_classes"}},
    model_dir=os.path.join(WORKSPACE, "{{.TrainClause.Save}}"))

def train_input_fn(features, labels, batch_size):
    dataset = tf.data.Dataset.from_tensor_slices((dict(features), labels))
    dataset = dataset.shuffle(1000).repeat().batch(batch_size)
    return dataset

classifier.train(
    input_fn=lambda:train_input_fn(X, Y, BATCHSIZE),
    steps=STEP)

print("Dumping sql parsed data ...")
with open(os.path.join(WORKSPACE, "{{.TrainClause.Save}}", SQL_PARSING_RESULT_FILE), "w") as f:
    f.write("""{{.JSON}}""")

print("Done training")
{{- else}}
with open(os.path.join(WORKSPACE, "{{.InferClause.Model}}", SQL_PARSING_RESULT_FILE)) as f:
    desc = json.load(f)

def eval_input_fn(features, labels, batch_size):
    dataset = tf.data.Dataset.from_tensor_slices((dict(features), labels))
    dataset = dataset.batch(batch_size)
    return dataset

classifier = tf.estimator.{{.TrainClause.Estimator}}(
        feature_columns=feature_columns,
        hidden_units=eval(desc["TrainClause"]["Attrs"]["hidden_units"]),
        n_classes=eval(desc["TrainClause"]["Attrs"]["n_classes"]),
        model_dir=os.path.join(WORKSPACE, "{{.InferClause.Model}}"))

eval_result = classifier.evaluate(
        input_fn=lambda:eval_input_fn(X, Y, BATCHSIZE),
        steps=STEP)
print("\nTest set accuracy: {accuracy:0.5f}\n".format(**eval_result))
{{- end}}
`

var codegen_template *template.Template = template.Must(template.New("codegen").Parse(codegen_template_text))
