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

__all__ = ['infer_feature_columns', 'get_ordered_field_descs']

import re

import numpy as np
import six
from runtime.feature.column import (CategoryIDColumn, EmbeddingColumn,
                                    IndicatorColumn, NumericColumn)
from runtime.feature.field_desc import DataFormat, DataType, FieldDesc
from runtime.verifier import fetch_samples


def init_column_map(target_fc_map, fc):
    """
    Init the target_fc_map by the feature column fc.

    Args:
        target_fc_map (dict[str -> FeatureColumn): the feature column map,
            where the key is the field name.
        fc (FeatureColumn): the feature column object.

    Returns:
        None.
    """
    if isinstance(fc, (EmbeddingColumn, IndicatorColumn)) \
            and len(fc.get_field_desc()) == 0:
        if fc.name not in target_fc_map:
            target_fc_map[fc.name] = []

        target_fc_map[fc.name].append(fc)
    else:
        for fd in fc.get_field_desc():
            if fd.name not in target_fc_map:
                target_fc_map[fd.name] = []

            target_fc_map[fd.name].append(fc)


def make_feature_column_map(features):
    """
    Build a FeatureColumn map by the features.

    Args:
        features (dict[str -> list[FeatureColumn]]): the
            input feature columns. The key of the dict is
            the target name, e.g. "feature_columns".

    Returns:
        A map of type dict[str -> dict[str -> list[FeatureColumn]]].
        The key of the outer dict is the target name, e.g. "feature_columns",
        and the key of the inner dict is the field name.
    """
    fc_map = {}
    for target, fc_list in features.items():
        if target not in fc_map:
            fc_map[target] = {}

        for fc in fc_list:
            init_column_map(fc_map[target], fc)

    return fc_map


def make_field_desc_map(features):
    """
    Build a FieldDesc dict by the features.

    Args:
        features (dict[str -> list[FeatureColumn]]): the
            input feature columns. The key of the dict is
            the target name, e.g. "feature_columns".

    Returns:
        A map of type dict[str -> FieldDesc], where the
        key is the field name.
    """
    fd_map = {}
    for _, fc_list in features.items():
        for fc in fc_list:
            for fd in fc.get_field_desc():
                fd_map[fd.name] = fd

    return fd_map


def new_default_field_desc(name):
    """
    Create a new default FieldDesc object.

    Args:
        name: the FieldDesc name.

    Returns:
        A FieldDesc object whose name is the given name,
        and the data type is INT.
    """
    return FieldDesc(name=name, dtype=DataType.INT64)


# A regular expression to match any real number
REAL_NUMBER_PATTERN = re.compile(
    "((\\+|-)?([0-9]+)(\\.[0-9]+)?)|((\\+|-)?\\.?[0-9]+)")

# A regular expression to match the form of "3,5,7"
CSV_PATTERN = re.compile(
    "\\s*((%s)\\s*\\,\\s*)+(%s)\\s*(\\,?)\\s*" %
    (REAL_NUMBER_PATTERN.pattern, REAL_NUMBER_PATTERN.pattern))

# A regular expression to match the form of "0:3.2 7:-2.3"
KV_PATTERN = re.compile("([0-9]+:(%s)\\s*)+" % REAL_NUMBER_PATTERN.pattern)

# A regular expression to match multiple blanks
BLANK_PATTERN = re.compile("\\s+")

# The Python 2/3 int64 type
INT64_TYPE = long if six.PY2 else int  # noqa: F821


def escape_delimiter(delimiter):
    if delimiter in ["|", ".", "+", "?", "*", "$"]:
        return "\\" + delimiter

    if delimiter == " ":
        return "\\s"

    return delimiter


def infer_string_data_format(str_data, delimiter="", delimiter_kv=""):
    """
    Infer the data format of the given string.

    Args:
        str_data (str): a given string.

    Returns:
        One of PLAIN, CSV and KV.
    """
    if CSV_PATTERN.fullmatch(str_data):
        return DataFormat.CSV

    if KV_PATTERN.fullmatch(str_data):
        return DataFormat.KV

    if delimiter and delimiter_kv:
        delimiter = escape_delimiter(delimiter)
        delimiter_kv = escape_delimiter(delimiter_kv)
        pattern = "((\\w|\\d)+(%s)?(%s)?(%s)?)+" % (
            delimiter_kv, REAL_NUMBER_PATTERN.pattern, delimiter)
        kv_regex = re.compile(pattern)
        if kv_regex.fullmatch(str_data):
            return DataFormat.KV

    return DataFormat.PLAIN


def fill_csv_field_desc(cell, field_desc):
    """
    Fill the FieldDesc info by the cell data in the CSV format,
    including shape, delimiter, max_id, dtype, etc. of the FieldDesc.

    Args:
        cell (str): the cell data of the table in the CSV format.
        field_desc (FieldDesc): the FieldDesc object.

    Returns:
        None.
    """
    raw_values = cell.split(",")
    values = []
    for v in raw_values:
        v = v.strip()
        if v:
            values.append(v)

    if field_desc.is_sparse:
        assert field_desc.shape is not None, \
            "the shape of CSV format data must be given"
    else:
        if field_desc.shape is None:
            field_desc.shape = [len(values)]

        size = np.prod(field_desc.shape)
        if np.prod(field_desc.shape) != len(values):
            if size > 1:
                raise ValueError(
                    "column %s should be csv format dense tensor "
                    "of %d element(s), but got %d element(s)" %
                    (field_desc.name, np.prod(field_desc.shape), len(values)))

            field_desc.shape = [len(values)]

    # FIXME(sneaxiy): currently, we only support sparse tensor in CSV format
    # whose values are 0 or 1. The numeric values in the cell data are the
    # indices where the values of the sparse tensor are 1. For example, the
    # cell value "3,5,7" indicates a sparse tensor x, and
    # x[3] = x[5] = x[7] = 1, and the other values of x are all zeros. Since
    # the index is always of integer type, we force to set the data type of
    # sparse tensor in CSV format is "Int". We should remove this constraint
    # if we will support other data formats in the future.
    if field_desc.is_sparse:
        field_desc.dtype = DataType.INT64

    field_desc.delimiter = ","
    for v in values:
        if field_desc.dtype == DataType.INT64:
            try:
                int_value = INT64_TYPE(v)
            except ValueError:
                field_desc.dtype = DataType.FLOAT32
                field_desc.max_id = 0  # clear the max id
                continue
        else:
            continue

        # INT type, record the maximum id
        field_desc.max_id = max(field_desc.max_id, int_value)


def fill_kv_field_desc(cell, field_desc):
    """
    Fill the FieldDesc info by the cell data in the KV format,
    including shape, etc. of the FieldDesc.

    Args:
        cell (str): the cell data of the table in the KV format.
        field_desc (FieldDesc): the FieldDesc object.

    Returns:
        None.
    """
    # TODO(sneaxiy): support other delimiter_kv in feature derivation
    if field_desc.delimiter_kv not in [None, ""]:
        return

    # split and remove empty string
    split = [s for s in BLANK_PATTERN.split(cell) if s]
    max_idx = field_desc.shape[0]
    for s in split:
        idx = INT64_TYPE(s.split(':', 2)[0]) + 1
        if idx > max_idx:
            max_idx = idx

    field_desc.shape[0] = max_idx


def fill_plain_field_desc(cell, field_desc):
    """
    Fill the FieldDesc info by the cell data in the PLAIN format,
    including shape, dtype, vocabulary, etc. of the FieldDesc.
    This method would try to convert the cell data to be an integer
    or floating-point number if possible.

    Args:
        cell (str): the cell data of the table in the PLAIN format.
        field_desc (FieldDesc): the FieldDesc object.

    Returns:
        None.
    """
    try:
        int_value = INT64_TYPE(cell)
    except ValueError:
        int_value = None

    if int_value is not None:
        field_desc.shape = [1]
        return

    try:
        float_value = float(cell)
    except ValueError:
        float_value = None

    if float_value is None:
        field_desc.dtype = DataType.STRING
        field_desc.shape = [1]
        if field_desc.vocabulary is None:
            field_desc.vocabulary = set()
        # Build vocabulary from the sample data
        field_desc.vocabulary.add(cell)
    else:
        field_desc.dtype = DataType.FLOAT32
        field_desc.shape = [1]


def fill_field_descs(generator, fd_map):
    """
    Fill the FieldDesc infos in the FieldDesc map by the
    generator data.

    Args:
        generator (generator): a generator which yields
            each row of the table data.
        fd_map (dict[str -> FieldDesc]): a FieldDesc map,
            where the key is the field name.

    Returns:
        None.
    """
    names = generator.field_names
    dtypes = generator.field_types
    str_column_indices = []
    for idx, dtype in enumerate(dtypes):
        dtype = dtype.upper()
        if dtype in ["INT", "TINYINT", "DECIMAL", "BIGINT"]:
            fd_map[names[idx]].dtype = DataType.INT64
            fd_map[names[idx]].shape = [1]
        elif dtype in ["FLOAT", "DOUBLE"]:
            fd_map[names[idx]].dtype = DataType.FLOAT32
            fd_map[names[idx]].shape = [1]
        elif dtype in ["CHAR", "VARCHAR", "TEXT", "STRING"]:
            str_column_indices.append(idx)
        else:
            raise ValueError("unsupported field type %s" % dtype)

    # No string column, just return
    if not str_column_indices:
        return

    original_size = {}
    for name, fd in fd_map.items():
        if fd.shape is None:
            original_size[name] = 1
        else:
            original_size[name] = np.prod(fd.shape)

    format = [None] * len(str_column_indices)
    field_descs = [fd_map[names[i]] for i in str_column_indices]
    for row_idx, row_data in enumerate(generator()):
        row_data = [row_data[i] for i in str_column_indices]
        if row_idx == 0:
            for i, cell in enumerate(row_data):
                format[i] = infer_string_data_format(
                    cell, field_descs[i].delimiter,
                    field_descs[i].delimiter_kv)
                field_descs[i].format = format[i]

        for i, cell in enumerate(row_data):
            if format[i] == DataFormat.PLAIN:
                fill_plain_field_desc(cell, field_descs[i])
            elif format[i] == DataFormat.CSV:
                fill_csv_field_desc(cell, field_descs[i])
            elif format[i] == DataFormat.KV:
                if original_size.get(field_descs[i].name, 1) == 1:
                    if row_idx == 0:
                        field_descs[i].shape = [1]

                    fill_kv_field_desc(cell, field_descs[i])
            else:
                raise ValueError("unsupported data format {}".format(
                    format[i]))


def update_feature_column(fc, fd_map):
    """
    Update the FeatureColumn object by the FieldDesc map.

    Args:
        fc (FeatureColumn): a FeatureColumn object. Only EmbeddingColumn
            and IndicatorColumn without category_column info would be
            updated currently.
        fd_map (dict[str -> FieldDesc]): a FieldDesc map, where the key is the
            field name.

    Returns:
        None.
    """
    if isinstance(fc, EmbeddingColumn) and fc.category_column is None:
        field_desc = fd_map[fc.name]
        if field_desc is None:
            raise ValueError("column not found or inferred: %s" % fc.name)

        # FIXME(typhoonzero): when to use sequence_category_id_column?
        # if column fieldDesc is SPARSE, the sparse shape should
        # be in cs.Shape[0]
        bucket_size = field_desc.shape[0]
        if not field_desc.is_sparse:
            assert field_desc.max_id > 0, \
                "use dense column on embedding column " \
                "but did not got a correct MaxID"
            bucket_size = field_desc.max_id + 1

        fc.category_column = CategoryIDColumn(field_desc, bucket_size)
        return

    if isinstance(fc, IndicatorColumn) and fc.category_column is None:
        field_desc = fd_map[fc.name]
        if field_desc is None:
            raise ValueError("column not found or inferred: %s" % fc.name)

        assert not field_desc.is_sparse, \
            "cannot use sparse column with indicator column"
        assert field_desc.max_id > 0, \
            "use indicator column but did not got a correct MaxID"
        bucket_size = field_desc.max_id + 1
        fc.category_column = CategoryIDColumn(field_desc, bucket_size)


def new_feature_column(field_desc):
    """
    Create a new FeatureColumn object by the given FieldDesc object.

    Args:
        field_desc (FieldDesc): a given FieldDesc object.

    Returns:
        If field_desc.dtype is STRING, return an EmbeddingColumn object.
        Otherwise, return a NumericColumn object.
    """
    if field_desc.dtype != DataType.STRING:
        return NumericColumn(field_desc)
    else:
        category_column = CategoryIDColumn(field_desc,
                                           len(field_desc.vocabulary))
        # NOTE(typhoonzero): a default embedding size of 128 is enough
        # for most cases.
        embedding = EmbeddingColumn(category_column=category_column,
                                    dimension=128,
                                    combiner="sum")
        embedding.name = field_desc.name
        return embedding


def derive_feature_columns(targets, fc_map, fd_map, selected_field_names,
                           label_name):
    """
    Derive the FeatureColumn.

    Args:
        targets (list[str]): the feature column targets,
            e.g. "feature_columns".
        fc_map (dict[str -> dict[str -> list[FeatureColumn]]]): a FeatureColumn
            map, where the key of the outer dict is the target name, e.g.
            "feature_columns", and the key of the inner dict is the field name.
        fd_map (dict[str -> FieldDesc]): a FieldDesc map, where the key is the
            field name.
        selected_field_names (list[str]): the selected field name of the SQL
            statement.
        label_name (str): the label name of the TO TRAIN statement.

    Returns:
        None.
    """
    for target in targets:
        if target not in fc_map:
            fc_map[target] = {}

        fc_target_map = fc_map[target]

        new_fc_target_map = {}  # field_name -> list(FeatureColumn)
        for field_name in fc_target_map:
            if field_name in selected_field_names:
                new_fc_target_map[field_name] = fc_target_map[field_name]
                continue

            if len(fc_map) > 1:
                raise ValueError("cannot expand '%s' in COLUMN clause",
                                 field_name)

            field_pattern = re.compile(field_name, flags=re.I)
            found = False
            for selected_field_name in selected_field_names:
                if not field_pattern.fullmatch(selected_field_name):
                    continue

                new_fc = fc_target_map[field_name][0].new_feature_column_from(
                    fd_map[selected_field_name])
                new_fc_target_map[selected_field_name] = [new_fc]
                found = True

            if not found:
                raise ValueError(
                    "'%s' in COLUMN clause does not match any selected fields"
                    % field_name)

            del fd_map[field_name]

        # ================== MAIN LOOP ==================
        # Update or generate FeatureColumn for each selected field:
        for selected_field_name in selected_field_names:
            if label_name == selected_field_name:
                continue  # ignore label field

            fc_list = new_fc_target_map.get(selected_field_name, None)
            if fc_list is not None:
                for fc in fc_list:
                    update_feature_column(fc, fd_map)
            else:
                if len(fc_map) > 1:
                    # if column clause have more than one target, each target
                    # should specify the full list of the columns to use.
                    continue

                field_desc = fd_map.get(selected_field_name, None)
                if field_desc is None:
                    raise ValueError("column not found or inferred: %s" %
                                     selected_field_name)
                new_fc = new_feature_column(field_desc)
                new_fc_target_map[selected_field_name] = [new_fc]

        fc_target_map.clear()
        fc_target_map.update(new_fc_target_map)


def update_ir_feature_columns(features, fc_map, selected_field_names,
                              label_name):
    """
    Update the IR FeatureColumn map `features` by the derived FeatureColumn map
    `fc_map` . If any FeatureColumn inside `fc_map` does not exist in
    `features`, it would be added to `features` . Notice that `features` is not
    updated in-place, and we would return a new updated IR FeatureColumn map in
    this method.

    Args:
        features (dict[str -> list[FeatureColumn]]): the input IR FeatureColumn
            map to be updated. The key of the dict is the target name, e.g.
            "feature_columns".
        fc_map (dict[str -> dict[str -> list[FeatureColumn]]]): a derived
            FeatureColumn map, where the key of the outer dict is the target
            name, e.g. "feature_columns", and the key of the inner dict is
            the field name.
        label_name (str): the label name of the TO TRAIN statement.
        selected_field_names (list[str]): the selected field name of the SQL
            statement.

    Returns:
        A new IR FeatureColumn map of dict[str -> list[FeatureColumn]], which
        is updated from the inputs `features` and `fc_map` .
    """
    new_ir_feature_columns = {}
    for target, target_fc_map in fc_map.items():
        new_fc_list = []
        for field_name in selected_field_names:
            if field_name == label_name:
                continue

            fc_list = target_fc_map.get(field_name, None)
            if fc_list is None:
                continue

            for fc in fc_list:
                if fc not in new_fc_list:
                    new_fc_list.append(fc)

        single_fd_fcs = []
        multi_fd_fcs = []
        for fc in new_fc_list:
            field_desc_num = len(fc.get_field_desc())
            assert field_desc_num > 0, "FieldDesc number must be larger than 0"
            if field_desc_num == 1:
                single_fd_fcs.append(fc)
            else:
                multi_fd_fcs.append(fc)

        if multi_fd_fcs:
            original_fc_list = features[target]
            indices = []
            for fc in multi_fd_fcs:
                found = False
                for i, original_fc in enumerate(original_fc_list):
                    if fc == original_fc:
                        indices.append(i)
                        found = True
                        break

                if not found:
                    raise ValueError("some feature column is missing in the "
                                     "derivation stage")

            sorted_pos = sorted(range(len(indices)), key=lambda k: indices[k])
            multi_fd_fcs = [multi_fd_fcs[i] for i in sorted_pos]

        new_fc_list = single_fd_fcs + multi_fd_fcs
        new_ir_feature_columns[target] = new_fc_list

    return new_ir_feature_columns


def derive_label(label, fd_map):
    """
    Derive the feature column of the label.

    Args:
        label (FeatureColumn): the FeatureColumn object of the label.
        fd_map: (dict[str -> FieldDesc]): a FieldDesc map, where the key is the
            field name.

    Returns:
        A derived NumericColumn of the label.
    """
    label_name = label.get_field_desc()[0].name if label is not None else None
    if not label_name:
        return  # NOTE: clustering model may not specify Label

    label_field_desc = fd_map[label_name]
    assert label_field_desc is not None, \
        "deriveLabel: LABEL COLUMN '%s' not found" % label_name

    # use shape [] if label shape is [1] for TensorFlow scalar label
    # shape should be [].
    shape = label_field_desc.shape
    if shape is None or (len(shape) == 1 and shape[0] == 1):
        label_field_desc.shape = []

    return NumericColumn(label_field_desc)


def infer_feature_columns(conn, select, features, label, n=1000):
    """
    Infer the FeatureColumns.

    Args:
        conn: the database connection object.
        select (str): the select SQL statement.
        features (dict[str -> list[FeatureColumn]]): the input feature
            columns. The key of the dict is the target name, e.g.
            "feature_columns".
        label (FeatureColumn): the FeatureColumn object of the label.
        n (int): the sample number to be fetched in the table. Default
            1000.

    Returns:
        A tuple of (new_features, new_label), which can be accepted by IR.
    """
    if features is None:
        features = {}

    fc_map = make_feature_column_map(features)
    fd_map = make_field_desc_map(features)

    generator = fetch_samples(conn, select, n)
    if generator is None:
        raise ValueError("empty dataset")

    selected_field_names = generator.field_names
    assert len(set(selected_field_names)) == len(selected_field_names), \
        "duplicate selected field names"

    for name in selected_field_names:
        if name not in fd_map:
            fd_map[name] = new_default_field_desc(name)

    fill_field_descs(generator, fd_map)
    label_name = label.get_field_desc()[0].name if label is not None else None

    targets = list(features.keys())
    if not targets:
        targets.append("feature_columns")

    derive_feature_columns(targets, fc_map, fd_map, selected_field_names,
                           label_name)
    features = update_ir_feature_columns(features, fc_map,
                                         selected_field_names, label_name)
    label = derive_label(label, fd_map)
    return features, label


def get_ordered_field_descs(features):
    assert isinstance(features, dict)
    fd_list = []
    for target in features:
        for fc in features[target]:
            for fd in fc.get_field_desc():
                fd_list.append(fd)
    return fd_list
