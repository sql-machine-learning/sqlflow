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
from sqlflow_submitter import explainer
from sqlflow_submitter.db import (buffered_db_writer, connect_with_data_source,
                                  db_generator)


def xgb_shap_dataset(datasource, select, feature_column_names, label_spec,
                     feature_specs):
    label_spec["feature_name"] = label_spec["name"]
    conn = connect_with_data_source(datasource)
    stream = db_generator(conn.driver, conn, select, feature_column_names,
                          label_spec, feature_specs)
    xs = pd.DataFrame(columns=feature_column_names)
    ys = pd.DataFrame(columns=[label_spec["name"]])
    i = 0
    for row in stream():
        xs.loc[i] = [item[0] for item in row[0]]
        ys.loc[i] = row[1]
        i += 1
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
            label_spec,
            summary_params,
            result_table="",
            is_pai=False,
            hdfs_namenode_addr="",
            hive_location="",
            hdfs_user="",
            hdfs_pass=""):
    feature_column_names = [k["name"] for k in feature_field_meta]
    feature_specs = {k['name']: k for k in feature_field_meta}
    x = xgb_shap_dataset(datasource, select, feature_column_names, label_spec,
                         feature_specs)

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
        explainer.plot_and_save(lambda: shap.decision_plot(
            expected_value,
            shap_interaction_values,
            x,
            show=False,
            feature_display_range=slice(None, -40, -1),
            alpha=1))
    else:
        explainer.plot_and_save(lambda: shap.summary_plot(
            shap_values, x, show=False, **summary_params))


def write_shap_values(shap_values, driver, conn, result_table,
                      feature_column_names, hdfs_namenode_addr, hive_location,
                      hdfs_user, hdfs_pass):
    with buffered_db_writer(driver, conn, result_table, feature_column_names,
                            100, hdfs_namenode_addr, hive_location, hdfs_user,
                            hdfs_pass) as w:
        for row in shap_values:
            w.write(list(row))
