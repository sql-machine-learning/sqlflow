package sql

import (
	"bytes"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	simpleSelect = `
SELECT sepal_length, sepal_width, petal_length, petal_width, species
FROM irisis
`
	simpleTrainSelect = simpleSelect + `
TRAIN DNNClassifier
WITH 
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN *
INTO
  my_dnn_model
;
`
	simpleInferSelect = simpleSelect + `INFER my_dnn_model;`
)

func TestCodeGenTrain(t *testing.T) {
	assert := assert.New(t)
	assert.NotPanics(func() {
		sqlParse(newLexer(simpleTrainSelect))
	})

	var tpl bytes.Buffer
	err := codegen_template.Execute(&tpl, parseResult)
	if err != nil {
		log.Println("executing template:", err)
	}

	assert.Equal(tpl.String(), `
import tensorflow as tf
import sys, json, os
import mysql.connector

WORKSPACE = "/tmp/"
SQL_PARSING_RESULT_FILE = "sqlflow.json"

BATCHSIZE = 1
STEP = 1000
USER = "root"
PASSWORD = "root"
HOST = "localhost"
DATABASE = "yang"

db = mysql.connector.connect(user=USER, passwd=PASSWORD, host=HOST, database=DATABASE)
cursor = db.cursor()
cursor.execute("""SELECT sepal_length, sepal_width, petal_length, petal_width, species FROM irisis;""")
field_names = [i[0] for i in cursor.description]
columns = map(list, zip(*cursor.fetchall()))

feature_columns = [tf.feature_column.numeric_column(key=key) for key in field_names[:-1]]
X = {field_names[i]: columns[i] for i in range(len(field_names) - 1)}
Y = columns[-1]


classifier = tf.estimator.DNNClassifier(
    feature_columns=feature_columns,
    hidden_units=[10, 20],
    n_classes=3,
    model_dir=os.path.join(WORKSPACE, "my_dnn_model"))

def train_input_fn(features, labels, batch_size):
    dataset = tf.data.Dataset.from_tensor_slices((dict(features), labels))
    dataset = dataset.shuffle(1000).repeat().batch(batch_size)
    return dataset

classifier.train(
    input_fn=lambda:train_input_fn(X, Y, BATCHSIZE),
    steps=STEP)

print("Dumping sql parsed data ...")
with open(os.path.join(WORKSPACE, "my_dnn_model", SQL_PARSING_RESULT_FILE), "w") as f:
    f.write("""{
"Extended": true,
"Train": true,
"StandardSelect": "SELECT sepal_length, sepal_width, petal_length, petal_width, species FROM irisis;",
"TrainClause": {
"Estimator": "DNNClassifier",
"Attrs": {
"hidden_units": "[10, 20]",
"n_classes": "3"
},
"columns": [
"*"
],
"Save": "my_dnn_model"
}
}""")

print("Done training")
`)
}

func TestCodeGenInfer(t *testing.T) {
	assert := assert.New(t)
	assert.NotPanics(func() {
		sqlParse(newLexer(simpleInferSelect))
	})

	var tpl bytes.Buffer
	err := codegen_template.Execute(&tpl, parseResult)
	if err != nil {
		log.Println("executing template:", err)
	}
	assert.Equal(tpl.String(), `
import tensorflow as tf
import sys, json, os
import mysql.connector

WORKSPACE = "/tmp/"
SQL_PARSING_RESULT_FILE = "sqlflow.json"

BATCHSIZE = 1
STEP = 1000
USER = "root"
PASSWORD = "root"
HOST = "localhost"
DATABASE = "yang"

db = mysql.connector.connect(user=USER, passwd=PASSWORD, host=HOST, database=DATABASE)
cursor = db.cursor()
cursor.execute("""SELECT sepal_length, sepal_width, petal_length, petal_width, species FROM irisis;""")
field_names = [i[0] for i in cursor.description]
columns = map(list, zip(*cursor.fetchall()))

feature_columns = [tf.feature_column.numeric_column(key=key) for key in field_names[:-1]]
X = {field_names[i]: columns[i] for i in range(len(field_names) - 1)}
Y = columns[-1]


with open(os.path.join(WORKSPACE, "my_dnn_model", SQL_PARSING_RESULT_FILE)) as f:
    desc = json.load(f)

def eval_input_fn(features, labels, batch_size):
    dataset = tf.data.Dataset.from_tensor_slices((dict(features), labels))
    dataset = dataset.batch(batch_size)
    return dataset

classifier = tf.estimator.DNNClassifier(
        feature_columns=feature_columns,
        hidden_units=eval(desc["TrainClause"]["Attrs"]["hidden_units"]),
        n_classes=eval(desc["TrainClause"]["Attrs"]["n_classes"]),
        model_dir=os.path.join(WORKSPACE, "my_dnn_model"))

eval_result = classifier.evaluate(
        input_fn=lambda:eval_input_fn(X, Y, BATCHSIZE),
        steps=STEP)
print("\nTest set accuracy: {accuracy:0.5f}\n".format(**eval_result))
`)
}
