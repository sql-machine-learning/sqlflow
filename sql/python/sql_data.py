import tensorflow as tf
import mysql.connector


def connect(user, passwd, host, port):
    """connect is a convenient shortcut to mysql.connector.connect. Also,
    it makes it reaonable to import mysql.connector in this file, so
    to make it self-complete as a template.

    """
    return mysql.connector.connect(user=user,
                                   passwd=passwd,
                                   host=host,
                                   port=port)


def load(db, slct, label, features):
    """load returns the training features and the labels. The features is
a dict from field names to data columns, and the label is a dict from
the label field name to the label data column.

    Args:
        db: returned from mysql.connector.connect()
        slct (str): SQL SELECT statement
        label (str): the label field name as a string.
        features (list of str or None): feature field names.

    Returns:
        (features, label): maps from feature/label field names to columns.

    """
    cursor = db.cursor()
    cursor.execute(slct)
    f = [i[0] for i in cursor.description]  # get field names.
    c = list(zip(*cursor.fetchall()))  # transpose rows into columns.
    d = dict(zip(f, c))  # dict from field names to columns.
    l = d.pop(label)
    if features != None:
        d = dict((k, d[k]) for k in features)
    return d, l


def feature_columns(features):
    """feature_columns returns a list of tf.feature_column.

    Note: currently, we are using a quick-and-hacky implementation
    that assumes all fields are numeric_columns, and not supporting
    feature derivations like tf.feature_column.cross_column.

    Args:
        features: returned from load

    Returns:
        list of tf.feature_column

    """
    return [tf.feature_column.numeric_column(key=k) for k in features.keys()]
