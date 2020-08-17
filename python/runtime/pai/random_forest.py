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


def get_train_random_forest_pai_cmd(model_name, data_table, model_attrs,
                                    feature_column_names, label_column):
    """Get a command to submit a KMeans training task to PAI

    Args:
        model_name: model name on PAI
        data_table: input data table name
        model_attrs: model attributes for KMeans
        feature_column_names: names of feature columns
        label_column: name of the label column

    Returns:
        A string which is a PAI cmd
    """
    # default use numTrees = 1
    tree_num = 1
    tree_num_attr = model_attrs["tree_num"]
    if isinstance(tree_num_attr, int):
        tree_num = tree_num_attr
    feature_cols = ",".join(feature_column_names)

    return '''pai -name randomforests -DinputTableName="%s" -DmodelName="%s"
    -DlabelColName="%s" -DfeatureColNames="%s" -DtreeNum="%d"''' % (
        data_table, model_name, label_column, feature_cols, tree_num)


def get_explain_random_forest_pai_cmd(datasource, model_name, data_table,
                                      result_table, label_column):
    """Get a command to submit a PAI RandomForest explain task

    Args:
        datasource: current datasoruce
        model_name: model name on PAI
        data_table: input data table name
        result_table: name of the result table, PAI will automatically
            create this table
        label_column: name of the label column

    Returns:
        A string which is a PAI cmd
    """
    # NOTE(typhoonzero): for PAI random forests predicting, we can not load
    # the TrainStmt since the model saving is fully done by PAI. We directly
    # use the columns in SELECT statement for prediction, error will be
    # reported by PAI job if the columns not match.
    if not label_column:
        return ("must specify WITH label_column when using "
                "pai random forest to explain models")

    conn = db.connect_with_data_source(datasource)
    schema = db.get_table_schema(conn, data_table)
    columns = [f[0] for f in schema]
    conn.execute("DROP TABLE IF EXISTS %s;" % result_table)
    return (
        """pai -name feature_importance -project algo_public """
        """-DmodelName="%s" -DinputTableName="%s"  -DoutputTableName="%s" """
        """-DlabelColName="%s" -DfeatureColNames="%s" """
    ) % (model_name, data_table, result_table, label_column, ",".join(columns))
