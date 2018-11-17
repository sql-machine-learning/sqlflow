import tensorflow as tf
import database
import sys, json, os

SQL_PARSING_RESULT_FILE = 'train.json'

# TODO(tonyyang-svail): Add make sql recognize the following
BATCHSIZE = 1
STEP = 1000

# TODO(tonyyang-svail): hard-coded user, passwd, etc
USER = "root"
PASSWORD = "root"
HOST = "localhost"
DATABASE = "yang"

def get_standard_sql(desc):
    assert(desc["extended"])

    return desc["standardSelect"]

def get_model(desc, feature_columns):
    assert(desc["extended"])

    model_type = desc["trainClause"]["estimator"]
    hyperparam = { x : eval(desc["trainClause"]["attrs"][x]) for x in desc["trainClause"]["attrs"] }
    model_dir = os.path.join(desc["trainClause"]["save"])

    assert(model_type == "DNNClassifier")
    assert(isinstance(hyperparam["n_classes"], int))
    assert(isinstance(hyperparam["hidden_units"], list))

    classifier = tf.estimator.DNNClassifier(
            feature_columns=feature_columns,
            model_dir=model_dir,
            **hyperparam)

    return classifier

def train_input_fn(features, labels, batch_size):
    dataset = tf.data.Dataset.from_tensor_slices((dict(features), labels))
    dataset = dataset.shuffle(1000).repeat().batch(batch_size)
    return dataset

def evail_input_fn(features, labels, batch_size):
    dataset = tf.data.Dataset.from_tensor_slices((dict(features), labels))
    dataset = dataset.batch(batch_size)
    return dataset

desc = json.load(sys.stdin)

field_names, columns = database.load_data(USER, PASSWORD, HOST, DATABASE, get_standard_sql(desc))

if desc['train']:
    feature_columns = [tf.feature_column.numeric_column(key=key) for key in field_names[:-1]]
    classifier = get_model(desc, feature_columns)

    X = {field_names[i]: columns[i] for i in range(len(field_names) - 1)}
    Y = columns[-1]
    classifier.train(
            input_fn=lambda:train_input_fn(X, Y, BATCHSIZE),
            steps=STEP)

    print("Dumping train model metadata...")
    with open(os.path.join(desc['trainClause']['save'], SQL_PARSING_RESULT_FILE), 'w') as f:
        f.write(json.dumps(desc))
    print("Done training")
else:
    with open(os.path.join(desc['inferClause']['model'], SQL_PARSING_RESULT_FILE)) as f:
        desc = json.load(f)

    feature_columns = [tf.feature_column.numeric_column(key=key) for key in field_names[:-1]]
    classifier = get_model(desc, feature_columns)

    X = {field_names[i]: columns[i] for i in range(len(field_names) - 1)}
    Y = columns[-1]
    eval_result = classifier.evaluate(
            input_fn=lambda:train_input_fn(X, Y, BATCHSIZE),
            steps=STEP)
    print("\nTest set accuracy: {accuracy:0.5f}\n".format(**eval_result))

