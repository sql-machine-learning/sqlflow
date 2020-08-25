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

from runtime import db
from runtime.diagnostics import SQLFlowDiagnostic

# default config for kmeans model attributes
default_attrs = {
    "center_count": 3,
    "idx_table_name": "",
    "excluded_columns": ""
}


def get_train_kmeans_pai_cmd(datasource, model_name, data_table, model_attrs,
                             feature_column_names):
    """Get a command to submit a KMeans training task to PAI

    Args:
        datasource: current datasoruce
        model_name: model name on PAI
        data_table: input data table name
        model_attrs: model attributes for KMeans
        feature_column_names: names of feature columns

    Returns:
        A string which is a PAI cmd
    """
    [
        model_attrs.update({k: v}) for k, v in default_attrs.items()
        if k not in model_attrs
    ]
    center_count = model_attrs["center_count"]
    idx_table_name = model_attrs["idx_table_name"]
    if not idx_table_name:
        raise SQLFlowDiagnostic("Need to set idx_table_name in WITH clause")
    exclude_columns = model_attrs["excluded_columns"].split(",")

    # selectedCols indicates feature columns used to clustering
    selected_cols = [
        fc for fc in feature_column_names if fc not in exclude_columns
    ]

    conn = db.connect_with_data_source(datasource)
    conn.execute("DROP TABLE IF EXISTS %s" % idx_table_name)

    return (
        """pai -name kmeans -project algo_public """
        """-DinputTableName=%s -DcenterCount=%d -DmodelName %s """
        """-DidxTableName=%s -DselectedColNames="%s" -DappendColNames="%s" """
    ) % (data_table, center_count, model_name, idx_table_name,
         ",".join(selected_cols), ",".join(feature_column_names))
