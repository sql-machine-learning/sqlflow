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
import six
from runtime.feature.column import (BucketColumn, CategoryHashColumn,
                                    CategoryIDColumn, CrossColumn,
                                    EmbeddingColumn, IndicatorColumn,
                                    NumericColumn, SeqCategoryIDColumn)
from runtime.feature.field_desc import DataType
from runtime.model import EstimatorType

__all__ = [
    'compile_ir_feature_columns',
]


def to_package_dtype(dtype, package):
    """
    Convert dtype to the data type accepted by the feature column
    implementation packages including TensorFlow and XGBoost.

    Args:
        dtype (DataType): one of INT, FLOAT and STRING.
        package (module): the Python package, including TensorFlow
            and XGBoost feature column packages.

    Returns:
        The data type accepted by the feature column implementation
        packages including TensorFlow and XGBoost.
    """
    if dtype == DataType.INT:
        return package.dtypes.int64

    if dtype == DataType.FLOAT:
        return package.dtypes.float32

    if dtype == DataType.STRING:
        return package.dtypes.string

    raise ValueError("unsupported data type {}".format(dtype))


def compile_feature_column(fc_ir, model_type, package):
    """
    Compile an IR FeatureColumn object to a runtime feature column object.

    Args:
        fc_ir (FeatureColumn): the IR FeatureColumn object.
        model_type (EstimatorType): one of TENSORFLOW and XGBOOST.
        package (module): the Python package corresponding to the model_type.

    Returns:
        A runtime feature column object.
    """
    fc_package = package.feature_column

    if isinstance(fc_ir, NumericColumn):
        fd = fc_ir.get_field_desc()[0]
        return fc_package.numeric_column(fd.name, shape=fd.shape)

    if isinstance(fc_ir, BucketColumn):
        source_fc = compile_feature_column(fc_ir.source_column, model_type,
                                           package)
        return fc_package.bucketized_column(source_fc,
                                            boundaries=fc_ir.boundaries)

    if isinstance(fc_ir, CategoryIDColumn):
        fd = fc_ir.get_field_desc()[0]
        if fd.vocabulary:
            return fc_package.categorical_column_with_vocabulary_list(
                key=fd.name, vocabulary_list=list(fd.vocabulary))
        else:
            return fc_package.categorical_column_with_identity(
                key=fd.name, num_buckets=fc_ir.bucket_size)

    if isinstance(fc_ir, SeqCategoryIDColumn):
        assert model_type != EstimatorType.XGBOOST, \
            "SEQ_CATEGORY_ID is not supported in XGBoost models"
        fd = fc_ir.get_field_desc()[0]
        return fc_package.sequence_categorical_column_with_identity(
            key=fd.name, num_buckets=fc_ir.bucket_size)

    if isinstance(fc_ir, CategoryHashColumn):
        fd = fc_ir.get_field_desc()[0]
        dtype = to_package_dtype(fd.dtype, package)
        return fc_package.categorical_column_with_hash_bucket(
            key=fd.name, hash_bucket_size=fc_ir.bucket_size, dtype=dtype)

    if isinstance(fc_ir, CrossColumn):
        assert model_type != EstimatorType.XGBOOST, \
            "CROSS is not supported in XGBoost models"
        key_strs = []
        for key in fc_ir.keys:
            if isinstance(key, six.string_types):
                key_strs.append(key)
            elif isinstance(key, NumericColumn):
                fd = key.get_field_desc()[0]
                size = np.prod(fd.shape) if fd.shape else 1
                assert size == 1, "CROSS does not support shape not equal to 1"
                key_strs.append(fd.name)
            else:
                raise ValueError(
                    "field in CROSS must be of FeatureColumn or string type")

        return fc_package.crossed_column(
            key_strs, hash_bucket_size=fc_ir.hash_bucket_size)

    if isinstance(fc_ir, EmbeddingColumn):
        assert model_type != EstimatorType.XGBOOST, \
            "EMBEDDING is not supported in XGBoost models"
        category_column = compile_feature_column(fc_ir.category_column,
                                                 model_type, package)
        return fc_package.embedding_column(category_column,
                                           dimension=fc_ir.dimension,
                                           combiner=fc_ir.combiner)

    if isinstance(fc_ir, IndicatorColumn):
        category_column = compile_feature_column(fc_ir.category_column,
                                                 model_type, package)
        return fc_package.indicator_column(category_column)

    raise ValueError("unsupport FeatureColumn %s" % type(fc_ir))


def compile_ir_feature_columns(ir_features, model_type):
    """
    Compile an IR FeatureColumn map to a runtime feature column map.

    Args:
        ir_features (dict[str -> list[FeatureColumn]]): the IR FeatureColumn
            map, where the key is the target name, e.g. "feature_columns",
            and the element inside the list is the IR FeatureColumn object.
        model_type (EstimatorType): one of TENSORFLOW and XGBOOST.

    Returns:
        A runtime feature column map, whose type is
        dict[str -> list[RuntimeFeatureColumn]].
    """
    if model_type == EstimatorType.TENSORFLOW:
        import tensorflow
        package = tensorflow
    elif model_type == EstimatorType.XGBOOST:
        import runtime.xgboost
        package = runtime.xgboost
        assert len(ir_features) == 1 and "feature_columns" in ir_features, \
            "XGBoost only supports 'feature_columns' as the feature target"
    else:
        raise ValueError("only support TensorFlow and XGBoost model")

    all_fcs = dict()
    for target, fc_list in ir_features.items():
        fcs = [
            compile_feature_column(fc, model_type, package) for fc in fc_list
        ]
        all_fcs[target] = fcs

    return all_fcs
