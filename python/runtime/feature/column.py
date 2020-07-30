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

import six
from runtime.feature.field_desc import DataType, FieldDesc


class FeatureColumn(object):
    """
    FeatureColumn corresponds to the COLUMN clause in the TO TRAIN statement.
    It is the base class of all feature column classes.
    """
    def get_field_desc(self):
        """
        Get the underlying FieldDesc object list that the feature
        column object holds.

        Returns:
            A list of the FieldDesc objects.
        """
        raise NotImplementedError()

    def new_feature_column_from(self, field_desc):
        """
        Create a new feature column object of the same type
        that holds the given FieldDesc object.

        Args:
            field_desc (FieldDesc): the given FieldDesc object.

        Returns:
            A new feature column object which is of the same type,
            and holds the given FieldDesc object.
        """
        raise NotImplementedError()


class CategoryColumn(FeatureColumn):
    """
    CategoryColumn corresponds to the categorical feature column.
    It is the base class of all categorical feature column classes.
    """
    def num_class(self):
        """
        Get the class number of the categorical feature column.

        Returns:
            An integer which represents the class number.
        """
        raise NotImplementedError()


class NumericColumn(FeatureColumn):
    """
    NumericColumn represents a dense or sparse numeric feature.

    Args:
        field_desc (FieldDesc): the underlying FieldDesc object that the
            NumericColumn object holds.
    """
    def __init__(self, field_desc):
        assert isinstance(field_desc, FieldDesc)
        self.field_desc = field_desc

    def get_field_desc(self):
        return [self.field_desc]

    def new_feature_column_from(self, field_desc):
        return NumericColumn(field_desc)


class BucketColumn(CategoryColumn):
    """
    BucketColumn represents a bucketized feature column.

    Args:
        source_column (NumericColumn): the underlying NumericColumn object.
        boundaries (list[int|float]): the boundaries of the buckets.
    """
    def __init__(self, source_column, boundaries):
        assert isinstance(
            source_column,
            NumericColumn), "source_column of BUCKET must be of numeric type"
        self.source_column = source_column
        self.boundaries = boundaries

    def get_field_desc(self):
        return self.source_column.get_field_desc()

    def new_feature_column_from(self, field_desc):
        source_column = self.source_column.new_feature_column_from(field_desc)
        return BucketColumn(source_column, self.boundaries)

    def num_class(self):
        return len(self.boundaries) + 1


class CategoryIDColumn(CategoryColumn):
    """
    CategoryIDColumn represents a categorical id feature column.

    Args:
        field_desc (FieldDesc): the underlying FieldDesc object.
        bucket_size (int): the bucket size.
    """
    def __init__(self, field_desc, bucket_size):
        assert isinstance(field_desc, FieldDesc)
        self.field_desc = field_desc
        self.bucket_size = bucket_size

    def get_field_desc(self):
        return [self.field_desc]

    def new_feature_column_from(self, field_desc):
        return CategoryIDColumn(field_desc, self.bucket_size)

    def num_class(self):
        return self.bucket_size


class CategoryHashColumn(CategoryColumn):
    """
    CategoryHashColumn represents a categorical hash feature column.

    Args:
        field_desc (FieldDesc): the underlying FieldDesc object.
        bucket_size (int): the bucket size for hashing.
    """
    def __init__(self, field_desc, bucket_size):
        assert isinstance(field_desc, FieldDesc)
        self.field_desc = field_desc
        self.bucket_size = bucket_size

    def get_field_desc(self):
        return [self.field_desc]

    def new_feature_column_from(self, field_desc):
        return CategoryHashColumn(field_desc, self.bucket_size)

    def num_class(self):
        return self.bucket_size


class SeqCategoryIDColumn(CategoryColumn):
    """
    SeqCategoryIDColumn represents a sequential categorical id feature column.

    Args:
        field_desc (FieldDesc): the underlying FieldDesc object.
        bucket_size (int): the bucket size.
    """
    def __init__(self, field_desc, bucket_size):
        assert isinstance(field_desc, FieldDesc)
        self.field_desc = field_desc
        self.bucket_size = bucket_size

    def get_field_desc(self):
        return [self.field_desc]

    def new_feature_column_from(self, field_desc):
        return SeqCategoryIDColumn(field_desc, self.bucket_size)

    def num_class(self):
        return self.bucket_size


class CrossColumn(CategoryColumn):
    """
    CrossColumn represents a crossed feature column.

    Args:
        keys (str|NumericColumn): the underlying feature column name or
            NumericColumn object.
        hash_bucket_size (int): the bucket size for hashing.
    """
    def __init__(self, keys, hash_bucket_size):
        for k in keys:
            assert isinstance(k, (six.string_types, NumericColumn)), \
                "keys of CROSS must be of either string or numeric type"

        self.keys = keys
        self.hash_bucket_size = hash_bucket_size

    def get_field_desc(self):
        descs = []
        for k in self.keys:
            if isinstance(k, six.string_types):
                descs.append(
                    FieldDesc(name=k, dtype=DataType.STRING, shape=[1]))
            elif isinstance(k, NumericColumn):
                descs.extend(k.get_field_desc())
            else:
                raise ValueError("unsupported type %s" % type(k))

        return descs

    def new_feature_column_from(self, field_desc):
        raise NotImplementedError("CROSS does not support apply_to method")

    def num_class(self):
        return self.hash_bucket_size


class EmbeddingColumn(FeatureColumn):
    """
    EmbeddingColumn represents an embedding feature column.

    Args:
        category_column (CategoryColumn): the underlying CategoryColumn object.
        dimension (int): the dimension of the embedding.
        combiner (str): how to reduce if there are multiple entries in a single
            row. Currently 'mean', 'sqrtn' and 'sum' are supported.
        initializer (str): the initializer of the embedding table.
        name (str): only used when category_column=None. In this case, the
            category_column would be filled automaticaly in the feature
            derivation stage.
    """
    def __init__(self,
                 category_column=None,
                 dimension=0,
                 combiner="",
                 initializer="",
                 name=""):
        if category_column is not None:
            assert isinstance(category_column, CategoryColumn)

        self.category_column = category_column
        self.dimension = dimension
        self.combiner = combiner
        self.initializer = initializer
        self.name = name

    def get_field_desc(self):
        if self.category_column is None:
            return []

        return self.category_column.get_field_desc()

    def new_feature_column_from(self, field_desc):
        if self.category_column is not None:
            category_column = self.category_column.new_feature_column_from(
                field_desc)
        else:
            category_column = None

        return EmbeddingColumn(category_column=category_column,
                               dimension=self.dimension,
                               combiner=self.combiner,
                               initializer=self.initializer,
                               name=self.name)


class IndicatorColumn(FeatureColumn):
    """
    IndicatorColumn represents the one-hot feature column.

    Args:
        category_column (CategoryColumn): the underlying CategoryColumn object.
        name (str): only used when category_column=None. In this case, the
            category_column would be filled automaticaly in the feature
            derivation stage.
    """
    def __init__(self, category_column=None, name=""):
        if category_column is not None:
            assert isinstance(category_column, CategoryColumn)

        self.category_column = category_column
        self.name = name

    def get_field_desc(self):
        if self.category_column is None:
            return []

        return self.category_column.get_field_desc()

    def new_feature_column_from(self, field_desc):
        if self.category_column is not None:
            category_column = self.category_column.new_feature_column_from(
                field_desc)
        else:
            category_column = None

        return IndicatorColumn(category_column, self.name)
