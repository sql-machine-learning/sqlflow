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
import seaborn as sns
import pandas as pd
import numpy as np
import tensorflow as tf
from sqlflow_submitter.db import connect_with_data_source
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

def analyze(datasource, estimator_cls, select, feature_columns, feature_column_names,
            feature_metas={}, label_meta={}, model_params={}, save="", is_pai=False, plot_type='bar'):
    if is_pai:
        FLAGS = define_tf_flags()
        model_params["model_dir"] = FLAGS.checkpointDir
    else:
        model_params['model_dir'] = save

    def _input_fn():
        if is_pai:
            dataset = pai_maxcompute_input_fn(pai_table, datasource, feature_column_names, feature_metas, label_meta)
        else:
            conn = connect_with_data_source(datasource)
            dataset = input_fn(select, conn, feature_column_names, feature_metas, label_meta)
        return dataset.batch(1).cache()

    model_params.update(feature_columns)
    estimator = estimator_cls(**model_params)
    result = estimator.experimental_predict_with_explanations(lambda:_input_fn())
    pred_dicts = list(result)
    df_dfc = pd.DataFrame([pred['dfc'] for pred in pred_dicts])
    eval(plot_type)(df_dfc)

# The following code is generally base on
# https://www.tensorflow.org/tutorials/estimator/boosted_trees_model_understanding
def bar(df_dfc):
    import matplotlib.pyplot as plt

    # Plot.
    dfc_mean = df_dfc.abs().mean()
    N = 8  # View top 8 features.
    sorted_ix = dfc_mean.abs().sort_values()[-N:].index  # Average and sort by absolute.
    ax = dfc_mean[sorted_ix].plot(kind='barh',
                                  color=sns_colors[1],
                                  title='Mean |directional feature contributions|',
                                  figsize=(15, 9))
    ax.grid(False, axis='y')

    plt.savefig('summary', bbox_inches='tight') 

    matplotlib.use('module://plotille_backend')
    import matplotlib.pyplot as plt
    import sys
    sys.stdout.isatty = lambda:True
    plt.savefig('summary', bbox_inches='tight')

def violin(df_dfc):
    import matplotlib.pyplot as plt

    # Initialize plot.
    fig, ax = plt.subplots(1, 1, figsize=(15, 9))
  
    # Plot.
    dfc_mean = df_dfc.abs().mean()
    N = 10  # View top 8 features.
    sorted_ix = dfc_mean.abs().sort_values()[-N:].index  # Average and sort by absolute.
  
    # Add contributions of entire distribution.
    parts=ax.violinplot([df_dfc[w] for w in sorted_ix],
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

    plt.savefig('summary', bbox_inches='tight') 

    matplotlib.use('module://plotille_backend')
    import matplotlib.pyplot as plt
    import sys
    sys.stdout.isatty = lambda:True
    plt.savefig('summary', bbox_inches='tight')


# Boilerplate code for plotting :)
def _get_color(value):
    """To make positive DFCs plot green, negative DFCs plot red."""
    green, red = sns.color_palette()[2:4]
    if value >= 0: return green
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
    t = plt.text(x_coord, y_coord + 1 - OFFSET, 'feature\nvalue',
    fontproperties=font, size=12)
