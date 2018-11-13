import tensorflow as tf
import database

USER = "root"
PASSWORD = "root"
HOST = "localhost"
DATABASE = "yang"
TABLE = "irisis"
DATA = [("sepal_length", [5.1, 5.0, 6.4]),
        ("sepal_width", [3.3, 2.3, 2.8]),
        ("petal_length", [1.7, 3.3, 5.6]),
        ("petal_width", [0.5, 1.0, 2.2]),
        ("species", [0, 1, 2])]

# The model consumes all the columns of the table
# The first n - 1 columns will be X
# The n th column will be Y
SQL_COMMAND = "SELECT * FROM {}".format(TABLE)
MODEL_TYPE = "DNNClassifier"
HYPERPARAM = {
        "hidden_units": [10, 10],
        "n_classes": 3}
BATCHSIZE = 2
STEP = 1000

database.create_table(USER, PASSWORD, HOST, DATABASE, TABLE, DATA)
field_names, columns = database.load_data(USER, PASSWORD, HOST, DATABASE, SQL_COMMAND)

my_feature_columns = [tf.feature_column.numeric_column(key=key) for key in field_names[:-1]]
classifier = tf.estimator.DNNClassifier(
        feature_columns=my_feature_columns,
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
