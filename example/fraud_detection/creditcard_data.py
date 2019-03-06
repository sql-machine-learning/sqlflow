# Reference:  https://github.com/tensorflow/models/blob/master/samples/core/get_started/iris_data.py
# which released under the Apache License 2.0
import pandas as pd
import tensorflow as tf

TRAIN_DATA_FILE = './creditcard.csv.train'
TEST_DATA_FILE = './creditcard.csv.test'

CSV_COLUMN_NAMES = ["Time"] + ["V"+str(i) for i in range(1,29)] + ["Amount", "Class"]


def load_data(y_name='Class'):
    """Returns the iris dataset as (train_x, train_y), (test_x, test_y)."""
    import os
    import errno
    for filename in (TRAIN_DATA_FILE, TEST_DATA_FILE):
        if not os.path.exists(filename):
            raise FileNotFoundError(errno.ENOENT, os.strerror(errno.ENOENT), filename)

    train = pd.read_csv(TRAIN_DATA_FILE, names=CSV_COLUMN_NAMES, header=0)
    train_x, train_y = train, train.pop(y_name)

    test = pd.read_csv(TEST_DATA_FILE, names=CSV_COLUMN_NAMES, header=0)
    test_x, test_y = test, test.pop(y_name)

    return (train_x, train_y), (test_x, test_y)


def train_input_fn(features, labels, batch_size):
    """An input function for training"""

    # Convert the inputs to a Dataset.
    dataset = tf.data.Dataset.from_tensor_slices((dict(features), labels))

    # Shuffle, repeat, and batch the examples.
    dataset = dataset.shuffle(1000).repeat().batch(batch_size)

    # Return the dataset.
    return dataset


def eval_input_fn(features, labels, batch_size):
    """An input function for evaluation or prediction"""
    features=dict(features)
    if labels is None:
        # No labels, use only features.
        inputs = features
    else:
        inputs = (features, labels)

    # Convert the inputs to a Dataset.
    dataset = tf.data.Dataset.from_tensor_slices(inputs)

    # Batch the examples
    assert batch_size is not None, "batch_size must not be None"
    dataset = dataset.batch(batch_size)

    # Return the dataset.
    return dataset
