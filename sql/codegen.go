package sql

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/go-sql-driver/mysql"
)

// TODO(tonyyang): This is currently a quick hack to map from SQL
// field types to feature types.  We will enhance it to support more
// complex cases like cross features.
var fieldTypeFeatureType = map[string]string{"FLOAT": "numeric_column"}

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
	Estimator string
	Attrs     map[string]string
	Save      string
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

func newFiller(pr *extendedSelect, fts fieldTypes, db *DB) (*filler, error) {
	r := &filler{
		Train:          pr.train,
		StandardSelect: pr.standardSelect.String(),
		modelConfig: modelConfig{
			Estimator: pr.estimator,
			Attrs:     make(map[string]string),
			Save:      pr.save}}
	for k, v := range pr.attrs {
		r.Attrs[k] = v.String()
	}

	for _, c := range pr.columns {
		typ, ok := fts.get(c.val)
		if !ok {
			return nil, fmt.Errorf("genTF: Cannot find type of field %s", c.val)
		}
		ct := columnType{Name: c.val, Type: fieldTypeFeatureType[typ]}
		r.X = append(r.X, ct)
	}
	typ, ok := fts.get(pr.label)
	if !ok {
		return nil, fmt.Errorf("genTF: Cannot find type of label %s", pr.label)
	}
	r.Y = columnType{Name: pr.label, Type: fieldTypeFeatureType[typ]}

	if !pr.train {
		r.TableName = strings.Join(strings.Split(pr.into, ".")[:2], ".")
	}

	switch db.driverName {
	case "mysql":
		cfg, err := mysql.ParseDSN(db.dataSourceName)
		if err != nil {
			return nil, err
		}
		r.Driver = "mysql"
		r.User = cfg.User
		r.Password = cfg.Passwd
		r.Host = strings.Split(cfg.Addr, ":")[0]
		r.Port = strings.Split(cfg.Addr, ":")[1]
	case "sqlite3":
		r.Driver = "sqlite3"
		r.Database = db.dataSourceName
	default:
		return nil, fmt.Errorf("sqlfow currently doesn't support DB %v", db.driverName)
	}

	return r, nil
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
import mysql.connector

# Disable Tensorflow INFO and WARNING logs
tf.logging.set_verbosity(tf.logging.ERROR)
` +
	// TODO(tonyyang-svail): remove hard coded BATCHSIZE, STEP
	`
BATCHSIZE = 1
STEP = 1000

{{if eq .Driver "mysql"}}
db = mysql.connector.connect(user="{{.User}}",
                             passwd="{{.Password}}",
                             host="{{.Host}}",
                             port={{.Port}}{{if eq .Database ""}}{{- else}}, database="{{.DATABASE}}"{{end}})
{{else}}
{{if eq .Driver "sqlite3"}}
db = sqlite3.connect({{.Database}})
{{else}}
raise ValueError("unrecognized database driver: {{.Driver}}")
{{end}}
{{end}}

cursor = db.cursor()
cursor.execute("""{{.StandardSelect}}""")
field_names = [i[0] for i in cursor.description]
columns = list(map(list, zip(*cursor.fetchall())))

feature_columns = [{{range .X}}tf.feature_column.{{.Type}}(key="{{.Name}}"),
    {{end}}]
feature_column_names = [{{range .X}}"{{.Name}}",
    {{end}}]

X = {name: columns[field_names.index(name)] for name in feature_column_names}
{{if .Train}}
Y = columns[field_names.index("{{.Y.Name}}")]
{{- end}}

classifier = tf.estimator.{{.Estimator}}(
    feature_columns=feature_columns,{{range $key, $value := .Attrs}}
    {{$key}} = {{$value}},{{end}}
    model_dir= "{{.Save}}")

{{if .Train}}
def train_input_fn(features, labels, batch_size):
    dataset = tf.data.Dataset.from_tensor_slices((dict(features), labels))
    dataset = dataset.shuffle(1000).repeat().batch(batch_size)
    return dataset

classifier.train(
    input_fn=lambda:train_input_fn(X, Y, BATCHSIZE),
    steps=STEP)

def eval_input_fn(features, labels, batch_size):
    dataset = tf.data.Dataset.from_tensor_slices((dict(features), labels))
    dataset = dataset.batch(batch_size)
    return dataset

eval_result = classifier.evaluate(
        input_fn=lambda:eval_input_fn(X, Y, BATCHSIZE),
        steps=STEP)
print("\nTraining set accuracy: {accuracy:0.5f}\n".format(**eval_result))

print("Done training")
{{- else}}
def eval_input_fn(features, batch_size):
    dataset = tf.data.Dataset.from_tensor_slices(dict(features))
    dataset = dataset.batch(batch_size)
    return dataset

predictions = classifier.predict(
        input_fn=lambda:eval_input_fn(X, BATCHSIZE))

X["{{.Y.Name}}"] = [p['class_ids'][0] for p in predictions]

def insert(table_name, X, db):
    length = [len(X[key]) for key in X]
    assert len(set(length)) == 1, "All the fields should have the same length"

    field_names = [key for key in X]
    # FIXME(tony): HIVE and ODPS use INSERT INTO TABLE ...
    sql = "INSERT INTO {} ({}) VALUES ({})".format(
            table_name, ",".join(field_names), ",".join(["%s" for _ in field_names]))
    val = []
    for i in range(length[0]):
        val.append(tuple([str(X[f][i]) for f in field_names]))

    cursor = db.cursor()
    cursor.executemany(sql, val)
    db.commit()

insert("{{.TableName}}", X, db)

print("Done predicting. Predict Table : {{.TableName}}")
{{- end}}
`

var codegenTemplate = template.Must(template.New("codegen").Parse(codegenTemplateText))
