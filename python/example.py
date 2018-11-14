import tensorflow as tf
import database
import sys, json, os

# TODO(tonyyang-svail): Add make sql recognize the following
BATCHSIZE = 1
STEP = 1000

# TODO(tonyyang-svail): 
USER = "root"
PASSWORD = "root"
HOST = "localhost"
DATABASE = "yang"
TABLE = "irisis"

# DATA = [("sepal_length", [5.1, 5.0, 6.4]),
#         ("sepal_width", [3.3, 2.3, 2.8]),
#         ("petal_length", [1.7, 3.3, 5.6]),
#         ("petal_width", [0.5, 1.0, 2.2]),
#         ("species", [0, 1, 2])]
# database.create_table(USER, PASSWORD, HOST, DATABASE, TABLE, DATA)

def parse_job_desc(json_input):
    data = json.load(sys.stdin)
    assert(data["extended"])
    assert(data["train"])

    sql_command = data["standardSelect"]
    model_type = data["trainClause"]["estimator"]
    hyperparam = { x : eval(data["trainClause"]["attrs"][x]) for x in data["trainClause"]["attrs"] }
    model_dir = os.path.join(data["trainClause"]["save"])

    assert(model_type == "DNNClassifier")
    assert(isinstance(hyperparam["n_classes"], int))
    assert(isinstance(hyperparam["hidden_units"], list))
    return sql_command, model_type, hyperparam, model_dir

SQL_COMMAND, MODEL_TYPE, HYPERPARAM, MODEL_DIR = parse_job_desc(sys.stdin)

field_names, columns = database.load_data(USER, PASSWORD, HOST, DATABASE, SQL_COMMAND)

my_feature_columns = [tf.feature_column.numeric_column(key=key) for key in field_names[:-1]]
classifier = tf.estimator.DNNClassifier(
        feature_columns=my_feature_columns,
        model_dir=MODEL_DIR,
        **HYPERPARAM)

train_x = {field_names[i]: columns[i] for i in range(len(field_names) - 1)}
train_y = columns[-1]
batch_size, steps = BATCHSIZE, STEP

def train_input_fn(features, labels, batch_size):
    dataset = tf.data.Dataset.from_tensor_slices((dict(features), labels))
    dataset = dataset.shuffle(1000).repeat().batch(batch_size)
    return dataset

classifier.train(
        input_fn=lambda:train_input_fn(train_x, train_y, batch_size),
        steps=steps)

eval_result = classifier.evaluate(
        input_fn=lambda:train_input_fn(train_x, train_y, batch_size),
        steps=steps)
print("\nTest set accuracy: {accuracy:0.5f}\n".format(**eval_result))
