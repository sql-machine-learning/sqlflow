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
# limitations under the License

import os

import tensorflow as tf
from runtime import db
from runtime.dbapi.paiio import PaiIOConnection
from runtime.feature.compile import compile_ir_feature_columns
from runtime.feature.derivation import get_ordered_field_descs
from runtime.model import EstimatorType, oss
from runtime.tensorflow import is_tf_estimator
from runtime.tensorflow.import_model import import_model
from runtime.tensorflow.load_model import pop_optimizer_and_loss
from runtime.tensorflow.predict import estimator_predict, keras_predict


def predict_step(datasource, select, data_table, result_table, label_column,
                 oss_model_path):
    """PAI TensorFlow prediction wrapper
    This function do some preparation for the local prediction, say,
    download the model from OSS, extract metadata and so on.

    Args:
        datasource: the datasource from which to get data
        select: data selection SQL statement
        data_table: tmp table which holds the data from select
        result_table: table to save prediction result
        label_column: prediction label column
        oss_model_path: the model path on OSS
    """

    try:
        tf.enable_eager_execution()
    except:  # noqa: E722
        pass

    (estimator, feature_column_names, feature_column_names_map, feature_metas,
     label_meta, model_params,
     feature_columns_code) = oss.load_metas(oss_model_path,
                                            "tensorflow_model_desc")

    fc_map_ir = feature_columns_code
    feature_columns = compile_ir_feature_columns(fc_map_ir,
                                                 EstimatorType.TENSORFLOW)
    field_descs = get_ordered_field_descs(fc_map_ir)
    feature_column_names = [fd.name for fd in field_descs]
    feature_metas = dict([(fd.name, fd.to_dict()) for fd in field_descs])

    is_estimator = is_tf_estimator(import_model(estimator))

    # Keras single node is using h5 format to save the model, no need to deal
    # with export model format. Keras distributed mode will use estimator, so
    # this is also needed.
    model_local_dir = oss_model_path.split("/")[-1]
    if is_estimator:
        oss.load_file(oss_model_path, "exported_path")
        # NOTE(typhoonzero): directory "model_save" is hardcoded in
        # codegen/tensorflow/codegen.go
        oss.load_dir("%s/%s" % (oss_model_path, model_local_dir))
    else:
        oss.load_dir(os.path.join(oss_model_path, "model_save"))

    _predict(datasource=datasource,
             estimator_string=estimator,
             select=select,
             result_table=result_table,
             feature_columns=feature_columns,
             feature_column_names=feature_column_names,
             feature_column_names_map=feature_column_names_map,
             train_label_name=label_meta["feature_name"],
             result_col_name=label_column,
             feature_metas=feature_metas,
             model_params=model_params,
             save=model_local_dir,
             batch_size=1,
             pai_table=data_table)


def _predict(datasource,
             estimator_string,
             select,
             result_table,
             feature_columns,
             feature_column_names,
             feature_column_names_map,
             train_label_name,
             result_col_name,
             feature_metas={},
             model_params={},
             save="",
             batch_size=1,
             pai_table=""):
    estimator = import_model(estimator_string)
    model_params.update(feature_columns)
    is_estimator = is_tf_estimator(estimator)

    conn = PaiIOConnection.from_table(pai_table)
    selected_cols = db.selected_cols(conn, None)
    predict_generator = db.db_generator(conn, None)

    pop_optimizer_and_loss(model_params)

    if not is_estimator:
        if not issubclass(estimator, tf.keras.Model):
            # functional model need field_metas parameter
            model_params["field_metas"] = feature_metas
        print("Start predicting using keras model...")
        keras_predict(estimator, model_params, save, result_table,
                      feature_column_names, feature_metas, train_label_name,
                      result_col_name, conn, predict_generator, selected_cols)
    else:
        model_params['model_dir'] = save
        print("Start predicting using estimator model...")
        estimator_predict(result_table, feature_column_names, feature_metas,
                          train_label_name, result_col_name, conn,
                          predict_generator, selected_cols)

    print("Done predicting. Predict table : %s" % result_table)
