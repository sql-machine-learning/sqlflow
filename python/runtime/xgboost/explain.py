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
from runtime import db, explainer
from runtime.dbapi.paiio import PaiIOConnection


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
                     label_meta,
                     feature_metas,
                     is_pai,
                     pai_explain_table,
                     transform_fn=None,
                     feature_column_code=""):
    if is_pai:
        # (TODO: lhw) we may specify pai_explain_table in datasoure
        # and discard the condition statement here
        conn = PaiIOConnection.from_table(pai_explain_table)
        stream = db.db_generator(conn, None, label_meta)
    else:
        conn = db.connect_with_data_source(datasource)
        stream = db.db_generator(conn, select, label_meta)
    selected_cols = db.selected_cols(conn, select)

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
        features = db.read_features_from_row(row,
                                             selected_cols,
                                             feature_column_names,
                                             feature_metas,
                                             is_xgboost=True)
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
        # the result column name would be "c_0", "c_1" and "c_2"
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
                        column_names.append('{}_{}'.format(
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
            label_meta,
            summary_params,
            explainer="TreeExplainer",
            result_table="",
            is_pai=False,
            pai_explain_table="",
            oss_dest=None,
            oss_ak=None,
            oss_sk=None,
            oss_endpoint=None,
            oss_bucket_name=None,
            transform_fn=None,
            feature_column_code=""):
    if explainer == "XGBoostExplainer":
        if result_table == "":
            raise ValueError("""XGBoostExplainer must use with INTO to output
result to a table.""")
        bst = xgb.Booster()
        bst.load_model("my_model")
        gain_map = bst.get_score(importance_type="gain")
        fscore_map = bst.get_fscore()
        if is_pai:
            from runtime.dbapi.paiio import PaiIOConnection
            conn = PaiIOConnection.from_table(result_table)
        else:
            conn = db.connect_with_data_source(datasource)

        all_feature_keys = list(gain_map.keys())
        all_feature_keys.sort()
        with db.buffered_db_writer(conn, result_table,
                                   ["feature", "fscore", "gain"], 100) as w:
            for fkey in all_feature_keys:
                row = [fkey, fscore_map[fkey], gain_map[fkey]]
                w.write(list(row))
    else:
        # when explainer is "" or "TreeExplainer" use SHAP by default.
        shap_explain(datasource,
                     select,
                     feature_field_meta,
                     feature_column_names,
                     label_meta,
                     summary_params,
                     result_table=result_table,
                     is_pai=is_pai,
                     pai_explain_table=pai_explain_table,
                     oss_dest=oss_dest,
                     oss_ak=oss_ak,
                     oss_sk=oss_sk,
                     oss_endpoint=oss_endpoint,
                     oss_bucket_name=oss_bucket_name,
                     transform_fn=transform_fn,
                     feature_column_code=feature_column_code)


def shap_explain(datasource,
                 select,
                 feature_field_meta,
                 feature_column_names,
                 label_meta,
                 summary_params,
                 result_table="",
                 is_pai=False,
                 pai_explain_table="",
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
                         label_meta,
                         feature_field_meta,
                         is_pai,
                         pai_explain_table,
                         transform_fn=transform_fn,
                         feature_column_code=feature_column_code)
    shap_values, shap_interaction_values, expected_value = xgb_shap_values(x)
    if result_table != "":
        if is_pai:
            from runtime.dbapi.paiio import PaiIOConnection
            conn = PaiIOConnection.from_table(result_table)
        else:
            conn = db.connect_with_data_source(datasource)
        # TODO(typhoonzero): the shap_values is may be a
        # list of shape [3, num_samples, num_features],
        # use the first dimension here, should find out
        # when to use the other two. When shap_values is
        # not a list it can be directly used.
        if isinstance(shap_values, list):
            to_write = shap_values[0]
        else:
            to_write = shap_values
        write_shap_values(to_write, conn, result_table, feature_column_names)

    if summary_params.get("plot_type") == "decision":
        explainer.plot_and_save(
            lambda: shap.decision_plot(expected_value,
                                       shap_interaction_values,
                                       x,
                                       show=False,
                                       feature_display_range=slice(
                                           None, -40, -1),
                                       alpha=1), oss_dest, oss_ak, oss_sk,
            oss_endpoint, oss_bucket_name)
    else:
        explainer.plot_and_save(
            lambda: shap.summary_plot(
                shap_values, x, show=False, **summary_params), oss_dest,
            oss_ak, oss_sk, oss_endpoint, oss_bucket_name)


def write_shap_values(shap_values, conn, result_table, feature_column_names):
    with db.buffered_db_writer(conn, result_table, feature_column_names,
                               100) as w:
        for row in shap_values:
            # NOTE(typhoonzero): assume all shap explain value are float, and
            # there's no INT or other types of values yet.
            row_float = [float(c) for c in row]
            w.write(list(row_float))
