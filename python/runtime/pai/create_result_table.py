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
from runtime.model import EstimatorType
from runtime.pai import table_ops


def create_predict_result_table(datasource, select, result_table, label_column,
                                train_label_column, model_type):
    """Create predict result table with given name and label column

    Args:
        datasource: current datasource
        select: sql statement to get prediction data set
        result_table: the table name to save result
        label_column: name of the label column, if not exist in select
            result, we will add a int column in the result table
        train_label_column: name of the label column when training
        model_type: type of model defined in runtime.model.oss
    """
    conn = db.connect_with_data_source(datasource)
    conn.execute("DROP TABLE IF EXISTS %s" % result_table)
    # PAI ml will create result table itself
    if model_type == EstimatorType.PAIML:
        return

    create_table_sql = "CREATE TABLE %s AS SELECT * FROM %s LIMIT 0" % (
        result_table, select)
    conn.execute(create_table_sql)

    # if label is not in data table, add a int column for it
    schema = db.get_table_schema(conn, result_table)
    col_type = "INT"
    for (name, ctype) in schema:
        if name == train_label_column or name == label_column:
            col_type = ctype
            break
    col_names = [col[0] for col in schema]
    if label_column not in col_names:
        conn.execute(
            conn, "ALTER TABLE %s ADD %s %s" %
            (result_table, label_column, col_type))
    if train_label_column != label_column and train_label_column in col_names:
        conn.execute(
            conn, "ALTER TABLE %s DROP COLUMN %s" %
            (result_table, train_label_column))


# (TODO: lhw) This function is a common tool for prediction
# on all platforms, we need to move it to a new file
def create_explain_result_table(datasource, data_table, result_table,
                                model_type, estimator, label_column):
    """Create explain result table from given datasource

    Args:
        datasource: current datasource
        data_table: input data table name
        result_table: table name to store the result
        model_type: type of the model to use
        estimator: estimator class if the model is TensorFlow estimator
        label_column: column name of the predict label
    """
    conn = db.connect_with_data_source(datasource)
    drop_stmt = "DROP TABLE IF EXISTS %s" % result_table
    conn.execute(drop_stmt)

    create_stmt = ""
    if model_type == EstimatorType.PAIML:
        return
    elif model_type == EstimatorType.TENSORFLOW:
        if estimator.startswith("BoostedTrees"):
            column_def = ""
            if conn.driver == "mysql":
                column_def = "(feature VARCHAR(255), dfc FLOAT, gain FLOAT)"
            else:
                # Hive & MaxCompute
                column_def = "(feature STRING, dfc STRING, gain STRING)"
            create_stmt = "CREATE TABLE IF NOT EXISTS %s %s;" % (result_table,
                                                                 column_def)
        else:
            if not label_column:
                raise SQLFlowDiagnostic(
                    "need to specify WITH label_col=lable_col_name "
                    "when explaining deep models")
            create_stmt = get_create_shap_result_sql(conn, data_table,
                                                     result_table,
                                                     label_column)
    elif model_type == EstimatorType.XGBOOST:
        if not label_column:
            raise SQLFlowDiagnostic(
                "need to specify WITH label_col=lable_col_name "
                "when explaining xgboost models")
        create_stmt = get_create_shap_result_sql(conn, data_table,
                                                 result_table, label_column)
    else:
        raise SQLFlowDiagnostic(
            "not supported modelType %d for creating Explain result table" %
            model_type)

    if not conn.execute(create_stmt):
        raise SQLFlowDiagnostic("Can't create explain result table")


def get_create_shap_result_sql(conn, data_table, result_table, label_column):
    """Get a sql statement which create a result table for SHAP

    Args:
        conn: a database connection
        data_table: table name to read data from
        result_table: result table name
        label_column: column name of label

    Returns:
        a sql statement to create SHAP result table
    """
    schema = db.get_table_schema(conn, data_table)
    fields = ["%s STRING" % f[0] for f in schema if f[0] != label_column]
    return "CREATE TABLE IF NOT EXISTS %s (%s)" % (result_table,
                                                   ",".join(fields))


def create_evaluate_result_table(datasource, result_table, metrics):
    """Create a table to hold the evaluation result

    Args:
        datasource: current datasource
        result_table: the table name to save result
        metrics: list of evaluation metrics names
    """
    table_ops.drop_tables([result_table], datasource)
    # Always add loss
    ext_metrics = ["loss"]
    if isinstance(metrics, list):
        ext_metrics.extend(metrics)
    fields = ["%s STRING" % m for m in ext_metrics]
    sql = "CREATE TABLE IF NOT EXISTS %s (%s);" % (result_table,
                                                   ",".join(fields))
    conn = db.connect_with_data_source(datasource)
    conn.execute(sql)
