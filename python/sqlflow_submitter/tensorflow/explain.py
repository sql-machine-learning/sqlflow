# Copyright 2019 The SQLFlow Authors. All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import os

import matplotlib
import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
import seaborn as sns
import tensorflow as tf
import shap
from sqlflow_submitter import explainer
from sqlflow_submitter.db import buffered_db_writer, connect_with_data_source

from .input_fn import input_fn, pai_maxcompute_input_fn

sns_colors = sns.color_palette('colorblind')
# Disable Tensorflow INFO and WARNING logs
os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'

try:
    import sqlflow_models
except:
    pass

# TODO(shendiaomo): Remove after we fully upgrade to TF2.0
TF_VERSION_2 = True
TF_VERSION_PARTS = tf.__version__.split(".")
if int(TF_VERSION_PARTS[0]) == 1:
    TF_VERSION_2 = False

# Disable Tensorflow INFO and WARNING logs
if TF_VERSION_2:
    import logging
    tf.get_logger().setLevel(logging.ERROR)
else:
    tf.logging.set_verbosity(tf.logging.ERROR)
    from .pai_distributed import define_tf_flags


def explain(datasource,
            estimator_cls,
            select,
            feature_columns,
            feature_column_names,
            feature_metas={},
            label_meta={},
            model_params={},
            save="",
            is_pai=False,
            plot_type='bar',
            result_table="",
            hdfs_namenode_addr="",
            hive_location="",
            hdfs_user="",
            hdfs_pass=""):
    if is_pai:
        FLAGS = define_tf_flags()
        model_params["model_dir"] = FLAGS.checkpointDir
    else:
        model_params['model_dir'] = save

    def _input_fn():
        if is_pai:
            dataset = pai_maxcompute_input_fn(pai_table, datasource,
                                              feature_column_names,
                                              feature_metas, label_meta)
        else:
            conn = connect_with_data_source(datasource)
            dataset = input_fn(select, conn, feature_column_names,
                               feature_metas, label_meta)
        return dataset.batch(1).cache()

    model_params.update(feature_columns)
    estimator = estimator_cls(**model_params)
    if estimator_cls in (tf.estimator.BoostedTreesClassifier, tf.estimator.BoostedTreesRegressor):
        explain_boosted_trees(estimator, _input_fn, plot_type, result_table,
                              hdfs_namenode_addr, hive_location, hdfs_user, hdfs_pass)
    else:
        shap_dataset  = pd.DataFrame(columns=feature_column_names)
        for i, (features, label) in enumerate(_input_fn()):
            shap_dataset.loc[i] = [item.numpy()[0][0] for item in features.values()]
        explain_dnns(estimator, shap_dataset, plot_type, result_table,
                     hdfs_namenode_addr, hive_location, hdfs_user, hdfs_pass)

def explain_boosted_trees(estimator, input_fn, plot_type, result_table, 
                          hdfs_namenode_addr, hive_location, hdfs_user, hdfs_pass):
    result = estimator.experimental_predict_with_explanations(input_fn)
    pred_dicts = list(result)
    df_dfc = pd.DataFrame([pred['dfc'] for pred in pred_dicts])
    dfc_mean = df_dfc.abs().mean()
    if result_table != "":
        conn = connect_with_data_source(datasource)
        gain = estimator.experimental_feature_importances(normalize=True)
        create_explain_result_table(conn, result_table)
        write_dfc_result(dfc_mean, gain, result_table, conn,
                         feature_column_names, hdfs_namenode_addr,
                         hive_location, hdfs_user, hdfs_pass)
    explainer.plot_and_save(lambda: eval(plot_type)(df_dfc))

def explain_dnns(estimator, shap_dataset, plot_type, result_table, 
                 hdfs_namenode_addr, hive_location, hdfs_user, hdfs_pass):
    def predict(d):
        def input_fn():
            return tf.data.Dataset.from_tensor_slices(dict(pd.DataFrame(d, columns=shap_dataset.columns))).batch(1)
        return np.array([p['probabilities'][0] for p in estimator.predict(input_fn)])
    shap_values = shap.KernelExplainer(predict, shap_dataset).shap_values(shap_dataset)
    explainer.plot_and_save(lambda: shap.summary_plot(shap_values, shap_dataset, show=False, plot_type=plot_type))

def create_explain_result_table(conn, result_table):
    column_clause = ""
    if conn.driver == "mysql":
        column_clause = "(feature VARCHAR(255), dfc float, gain float)"
    else:
        column_clause = "(feature STRING, dfc float, gain float)"
    sql = "CREATE TABLE IF NOT EXISTS %s %s" % (result_table, column_clause)
    cursor = conn.cursor()
    try:
        cursor.execute("DROP TABLE IF EXISTS %s" % result_table)
        cursor.execute(sql)
        conn.commit()
    finally:
        cursor.close()


def write_dfc_result(dfc_mean, gain, result_table, conn, feature_column_names,
                     hdfs_namenode_addr, hive_location, hdfs_user, hdfs_pass):
    with buffered_db_writer(conn.driver, conn, result_table,
                            ["feature", "dfc", "gain"], 100,
                            hdfs_namenode_addr, hive_location, hdfs_user,
                            hdfs_pass) as w:
        for row_name in feature_column_names:
            w.write([row_name, dfc_mean.loc[row_name], gain[row_name]])


# The following code is generally base on
# https://www.tensorflow.org/tutorials/estimator/boosted_trees_model_understanding


def bar(df_dfc):
    # Plot.
    dfc_mean = df_dfc.abs().mean()
    N = 8  # View top 8 features.
    # Average and sort by absolute.
    sorted_ix = dfc_mean.abs().sort_values()[-N:].index
    ax = dfc_mean[sorted_ix].plot(
        kind='barh',
        color=sns_colors[1],
        title='Mean |directional feature contributions|',
        figsize=(15, 9))
    ax.grid(False, axis='y')


def violin(df_dfc):
    # Initialize plot.
    fig, ax = plt.subplots(1, 1, figsize=(15, 9))

    # Plot.
    dfc_mean = df_dfc.abs().mean()
    N = 10  # View top 8 features.
    # Average and sort by absolute.
    sorted_ix = dfc_mean.abs().sort_values()[-N:].index

    # Add contributions of entire distribution.
    parts = ax.violinplot([df_dfc[w] for w in sorted_ix],
                          vert=False,
                          showextrema=False,
                          showmeans=False,
                          showmedians=False,
                          widths=0.7,
                          positions=np.arange(len(sorted_ix)))
    plt.setp(parts['bodies'], facecolor='darkblue', edgecolor='black')
    ax.set_yticks(np.arange(len(sorted_ix)))
    ax.set_yticklabels(sorted_ix, size=16)
    ax.set_xlabel('Contribution to predicted probability', size=18)
    ax.grid(False, axis='y')
    ax.grid(True, axis='x')


# Boilerplate code for plotting :)
def _get_color(value):
    """To make positive DFCs plot green, negative DFCs plot red."""
    green, red = sns.color_palette()[2:4]
    if value >= 0:
        return green
    return red


def _add_feature_values(feature_values, ax):
    """Display feature's values on left of plot."""
    x_coord = ax.get_xlim()[0]
    OFFSET = 0.15
    for y_coord, (feat_name, feat_val) in enumerate(feature_values.items()):
        t = plt.text(x_coord, y_coord - OFFSET, '{}'.format(feat_val), size=12)
        t.set_bbox(dict(facecolor='white', alpha=0.5))
    from matplotlib.font_manager import FontProperties
    font = FontProperties()
    font.set_weight('bold')
    t = plt.text(x_coord,
                 y_coord + 1 - OFFSET,
                 'feature\nvalue',
                 fontproperties=font,
                 size=12)
