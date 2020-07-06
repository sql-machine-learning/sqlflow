# Copyright 2020 The SQLFlow Authors. All rights reserved.
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

import numpy as np
import pandas as pd
import scipy
import shap
import six
import xgboost as xgb
from sqlflow_runtime import db, explainer


def infer_dtype(feature):
    if isinstance(feature, np.ndarray):
        if feature.dtype == np.float32 or feature.dtype == np.float64:
            return 'float32'
        elif feature.dtype == np.int32 or feature.dtype == np.int64:
            return 'int64'
        else:
            raise ValueError('Not supported data type {}'.format(
                feature.dtype))
    elif isinstance(feature, (np.float32, np.float64, float)):
        return 'float32'
    elif isinstance(feature, (np.int32, np.int64, six.integer_types)):
        return 'int64'
    else:
        raise ValueError('Not supported data type {}'.format(type(feature)))


def xgb_shap_dataset(datasource,
                     select,
                     feature_column_names,
                     label_spec,
                     feature_specs,
                     is_pai,
                     pai_explain_table,
                     transform_fn=None,
                     feature_column_code=""):
    label_column_name = label_spec["feature_name"]
    if is_pai:
        pai_table_parts = pai_explain_table.split(".")
        formatted_pai_table = "odps://%s/tables/%s" % (pai_table_parts[0],
                                                       pai_table_parts[1])
        stream = db.pai_maxcompute_db_generator(formatted_pai_table,
                                                feature_column_names,
                                                label_column_name,
                                                feature_specs)
        selected_cols = db.pai_selected_cols(formatted_pai_table)
    else:
        conn = db.connect_with_data_source(datasource)
        stream = db.db_generator(conn.driver, conn, select,
                                 feature_column_names, label_spec,
                                 feature_specs)
        selected_cols = db.selected_cols(conn.driver, conn, select)

    if transform_fn:
        feature_names = transform_fn.get_feature_column_names()
    else:
        feature_names = feature_column_names

    xs = None
    dtypes = []
    sizes = []
    offsets = []

    i = 0
    for row, label in stream():
        features = db.read_features_from_row(row, selected_cols,
                                             feature_column_names,
                                             feature_specs)
        if transform_fn:
            features = transform_fn(features)

        flatten_features = []
        for j, feature in enumerate(features):
            if len(feature) == 3:  # convert sparse to dense
                col_indices, values, dense_shape = feature
                size = int(np.prod(dense_shape))
                row_indices = np.zeros(shape=[col_indices.size])
                sparse_matrix = scipy.sparse.csr_matrix(
                    (values, (row_indices, col_indices)), shape=[1, size])
                values = sparse_matrix.toarray()
            else:
                values = feature[0]

            if isinstance(values, np.ndarray):
                flatten_features.extend(values.flatten().tolist())
                if i == 0:
                    sizes.append(values.size)
                    dtypes.append(infer_dtype(values))
            else:
                flatten_features.append(values)
                if i == 0:
                    sizes.append(1)
                    dtypes.append(infer_dtype(values))

        # Create the column name according to the feature number
        # of each column.
        #
        # If the column "c" contains only 1 feature, the result
        # column name would be "c" too.
        #
        # If the column "c" contains 3 features,
        # the result column name would be "c-0", "c-1" and "c-2"
        if i == 0:
            offsets = np.cumsum([0] + sizes)
            column_names = []
            for j in six.moves.range(len(offsets) - 1):
                start = offsets[j]
                end = offsets[j + 1]
                if end - start == 1:
                    column_names.append(feature_names[j])
                else:
                    for k in six.moves.range(start, end):
                        column_names.append('{}-{}'.format(
                            feature_names[j], k))

            xs = pd.DataFrame(columns=column_names)

        xs.loc[i] = flatten_features

        i += 1
    # NOTE(typhoonzero): set dtype to the feature's actual type, or the dtype
    # may be "object". Use below code to reproduce:
    # import pandas as pd
    # feature_column_names=["a", "b"]
    # xs = pd.DataFrame(columns=feature_column_names)
    # for i in range(10):
    #     xs.loc[i] = [int(j) for j in range(2)]
    # print(xs.dtypes)
    columns = xs.columns
    for i, dtype in enumerate(dtypes):
        for j in six.moves.range(offsets[i], offsets[i + 1]):
            xs[columns[j]] = xs[columns[j]].astype(dtype)

    return xs


def xgb_shap_values(x):
    bst = xgb.Booster()
    bst.load_model("my_model")
    explainer = shap.TreeExplainer(bst)
    return explainer.shap_values(x), explainer.shap_interaction_values(
        x), explainer.expected_value


def explain(datasource,
            select,
            feature_field_meta,
            feature_column_names,
            label_spec,
            summary_params,
            result_table="",
            is_pai=False,
            pai_explain_table="",
            hdfs_namenode_addr="",
            hive_location="",
            hdfs_user="",
            hdfs_pass="",
            oss_dest=None,
            oss_ak=None,
            oss_sk=None,
            oss_endpoint=None,
            oss_bucket_name=None,
            transform_fn=None,
            feature_column_code=""):
    x = xgb_shap_dataset(datasource,
                         select,
                         feature_column_names,
                         label_spec,
                         feature_field_meta,
                         is_pai,
                         pai_explain_table,
                         transform_fn=transform_fn,
                         feature_column_code=feature_column_code)

    shap_values, shap_interaction_values, expected_value = xgb_shap_values(x)

    if result_table != "":
        if is_pai:
            # TODO(typhoonzero): the shape of shap_values is (3, num_samples, num_features)
            # use the first dimension here, should find out how to use the other two.
            write_shap_values(shap_values[0], "pai_maxcompute", None,
                              result_table, feature_column_names,
                              hdfs_namenode_addr, hive_location, hdfs_user,
                              hdfs_pass)
        else:
            conn = connect_with_data_source(datasource)
            write_shap_values(shap_values[0], conn.driver, conn, result_table,
                              feature_column_names, hdfs_namenode_addr,
                              hive_location, hdfs_user, hdfs_pass)
        return

    if summary_params.get("plot_type") == "decision":
        explainer.plot_and_save(
            lambda: shap.decision_plot(expected_value,
                                       shap_interaction_values,
                                       x,
                                       show=False,
                                       feature_display_range=slice(
                                           None, -40, -1),
                                       alpha=1), is_pai, oss_dest, oss_ak,
            oss_sk, oss_endpoint, oss_bucket_name)
    else:
        explainer.plot_and_save(
            lambda: shap.summary_plot(
                shap_values, x, show=False, **summary_params), is_pai,
            oss_dest, oss_ak, oss_sk, oss_endpoint, oss_bucket_name)


def write_shap_values(shap_values, driver, conn, result_table,
                      feature_column_names, hdfs_namenode_addr, hive_location,
                      hdfs_user, hdfs_pass):
    with db.buffered_db_writer(driver, conn, result_table,
                               feature_column_names, 100, hdfs_namenode_addr,
                               hive_location, hdfs_user, hdfs_pass) as w:
        for row in shap_values:
            w.write(list(row))
