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

import pandas as pd
import shap
import xgboost as xgb
from sqlflow_submitter import explainer
from sqlflow_submitter.db import connect_with_data_source, db_generator


def xgb_shap_dataset(datasource, select, feature_column_names, label_name,
                     feature_specs):
    conn = connect_with_data_source(datasource)
    stream = db_generator(conn.driver, conn, select, feature_column_names,
                          label_name, feature_specs)
    xs = pd.DataFrame(columns=feature_column_names)
    ys = pd.DataFrame(columns=[label_name])
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


def explain(datasource, select, feature_field_meta, label_name,
            summary_params):
    feature_column_names = [k["name"] for k in feature_field_meta]
    feature_specs = {k['name']: k for k in feature_field_meta}
    x = xgb_shap_dataset(datasource, select, feature_column_names, label_name,
                         feature_specs)

    shap_values, shap_interaction_values, expected_value = xgb_shap_values(x)

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
