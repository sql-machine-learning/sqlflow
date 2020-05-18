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
import shap
import xgboost as xgb
from sqlflow_submitter import db, explainer


def xgb_shap_dataset(datasource, select, feature_column_names, label_spec,
                     feature_specs, is_pai, pai_explain_table):
    label_column_name = label_spec["feature_name"]
    if is_pai:
        pai_table_parts = pai_explain_table.split(".")
        formatted_pai_table = "odps://%s/tables/%s" % (pai_table_parts[0],
                                                       pai_table_parts[1])
        stream = db.pai_maxcompute_db_generator(formatted_pai_table,
                                                feature_column_names,
                                                label_column_name,
                                                feature_specs)
        selected_cols = feature_column_names[:]
    else:
        conn = db.connect_with_data_source(datasource)
        stream = db.db_generator(conn.driver, conn, select,
                                 feature_column_names, label_spec,
                                 feature_specs)
        selected_cols = db.selected_cols(conn.driver, conn, select)

    xs = pd.DataFrame(columns=feature_column_names)
    i = 0
    for row, label in stream():
        features = db.read_features_from_row(row, selected_cols,
                                             feature_column_names,
                                             feature_specs)
        xs.loc[i] = [item[0] for item in features]
        i += 1
    # NOTE(typhoonzero): set dtype to the feature's actual type, or the dtype
    # may be "object". Use below code to reproduce:
    # import pandas as pd
    # feature_column_names=["a", "b"]
    # xs = pd.DataFrame(columns=feature_column_names)
    # for i in range(10):
    #     xs.loc[i] = [int(j) for j in range(2)]
    # print(xs.dtypes)
    for fname in feature_column_names:
        dtype = feature_specs[fname]["dtype"]
        xs[fname] = xs[fname].astype(dtype)
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
            oss_bucket_name=None):
    x = xgb_shap_dataset(datasource, select, feature_column_names, label_spec,
                         feature_field_meta, is_pai, pai_explain_table)

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
