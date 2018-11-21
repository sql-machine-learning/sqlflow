package sql

import (
	"text/template"
)

var fieldTypeFeatureType = map[string]string{"float": "numeric_column"}

type columnType struct {
	Name string
	Type string
}

type connectionConfig struct {
	User     string
	Password string
	Host     string
	Database string
	WorkDir  string
}

type TemplateFiller struct {
	Train bool
	// Model Config
	StandardSelect string
	Estimator      string
	Attrs          map[string]string
	Save           string
	// Data Config
	X []columnType
	Y columnType
	// Connection Config
	connectionConfig
}

func NewTemplateFiller(pr *extendedSelect, fts fieldTypes, cfg connectionConfig) (*TemplateFiller, bool) {
	r := &TemplateFiller{
		Train:          pr.train,
		StandardSelect: pr.standardSelect.String(),
		Estimator:      pr.estimator,
		Attrs:          make(map[string]string),
		Save:           pr.save}
	for k, v := range pr.attrs {
		r.Attrs[k] = v.String()
	}
	for _, c := range pr.columns {
		typ, ok := fts.get(c.val)
		if !ok {
			return nil, ok
		}
		ct := columnType{Name: c.val, Type: fieldTypeFeatureType[typ]}
		r.X = append(r.X, ct)
	}
	typ, ok := fts.get(pr.label)
	if !ok {
		return nil, ok
	}
	r.Y = columnType{Name: pr.label, Type: fieldTypeFeatureType[typ]}
	r.connectionConfig = cfg
	return r, true
}

const codegen_template_text = `
import tensorflow as tf
import sys, json, os
import mysql.connector
` +
	// TODO(tonyyang-svail): remove hard coded BATCHSIZE, STEP
	`
BATCHSIZE = 1
STEP = 1000

WORK_DIR = "{{.WorkDir}}"
USER = "{{.User}}"
PASSWORD = "{{.Password}}"
HOST = "{{.Host}}"
DATABASE = "{{.Database}}"

db = mysql.connector.connect(user=USER, passwd=PASSWORD, host=HOST, database=DATABASE)
cursor = db.cursor()
cursor.execute("""{{.StandardSelect}}""")
field_names = [i[0] for i in cursor.description]
columns = map(list, zip(*cursor.fetchall()))

feature_columns = [{{range .X}}tf.feature_column.{{.Type}}(key="{{.Name}}"),
    {{end}}]
feature_column_names = [{{range .X}}"{{.Name}}",
    {{end}}]

X = {name: columns[field_names.index(name)] for name in feature_column_names}
Y = columns[field_names.index("{{.Y.Name}}")]

{{if .Train}}
classifier = tf.estimator.{{.Estimator}}(
    feature_columns=feature_columns,
    hidden_units={{index .Attrs "hidden_units"}},
    n_classes={{index .Attrs "n_classes"}},
    model_dir=os.path.join(WORK_DIR, "{{.Save}}"))

def train_input_fn(features, labels, batch_size):
    dataset = tf.data.Dataset.from_tensor_slices((dict(features), labels))
    dataset = dataset.shuffle(1000).repeat().batch(batch_size)
    return dataset

classifier.train(
    input_fn=lambda:train_input_fn(X, Y, BATCHSIZE),
    steps=STEP)
` +
	// TODO(tonyyang-svail): avoid JSON
	// print("Dumping sql parsed data ...")
	// with open(os.path.join(WORK_DIR, "{{.Save}}", SQL_PARSING_RESULT_FILE), "w") as f:
	//     f.write("""{{.JSON}}""")
	`
print("Done training")
{{- else}}
` +
	// TODO(tonyyang-svail): avoid JSON
	// with open(os.path.join(WORK_DIR, "{{.InferClause.Model}}", SQL_PARSING_RESULT_FILE)) as f:
	//     desc = json.load(f)
	`
def eval_input_fn(features, labels, batch_size):
    dataset = tf.data.Dataset.from_tensor_slices((dict(features), labels))
    dataset = dataset.batch(batch_size)
    return dataset
` +
	// TODO(tonyyang-svail): remove hard coded DNNClassifier
	`
classifier = tf.estimator.DNNClassifier(
        feature_columns=feature_columns,
        hidden_units=eval(desc["TrainClause"]["Attrs"]["hidden_units"]),
        n_classes=eval(desc["TrainClause"]["Attrs"]["n_classes"]),
        model_dir=os.path.join(WORK_DIR, "{{.InferClause.Model}}"))

eval_result = classifier.evaluate(
        input_fn=lambda:eval_input_fn(X, Y, BATCHSIZE),
        steps=STEP)
print("\nTest set accuracy: {accuracy:0.5f}\n".format(**eval_result))
{{- end}}
`

var codegen_template *template.Template = template.Must(template.New("codegen").Parse(codegen_template_text))
