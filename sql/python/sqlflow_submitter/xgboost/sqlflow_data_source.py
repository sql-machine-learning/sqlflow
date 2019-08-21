# Copyright 2019 The SQLFlow Authors. All rights reserved.
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

import json
import typing
from typing import Iterator

from launcher.data_units import RecordBuilder

from .common import XGBoostError
from ..db import connect, db_generator, buffered_db_writer
from launcher import DataSource, config_fields, XGBoostResult, XGBoostRecord


class FeatureMeta(typing.NamedTuple):
    feature_name: str
    dtype: str = 'string'
    delimiter: str = ''
    shape: typing.List[int] = [1]
    is_sparse: bool = False
    fc_code: str = None

    @classmethod
    def convert_shape(cls, value) -> typing.List:
        if isinstance(value, str):
            shape = eval(value)
            assert isinstance(shape, typing.List)
            return shape
        elif isinstance(value, typing.List):
            return value
        else:
            raise XGBoostError('invalid shape %s of FeatureMeta' % value)


class SQLFlowDSConfig(typing.NamedTuple):
    is_train: bool
    standard_select: str
    db_config: typing.Dict
    is_tf_integrated: bool
    # FeatureMetas are useless temporarily, since tf transformation is not supported.
    x: typing.List[typing.Dict]
    label: typing.Dict = None
    group: typing.Dict = None
    weight: typing.Dict = None
    output_table: str = None
    write_batch_size: int = 1024


class SQLFlowDataSource(DataSource):
    def __init__(self, rank: int, num_worker: int,
                 column_conf: config_fields.ColumnFields,
                 source_conf):
        super().__init__(rank, num_worker, column_conf, source_conf)
        if not isinstance(source_conf, SQLFlowDSConfig):
            raise XGBoostError("SQLFlowDataSource: invalid source conf")

        # TODO: support tf.feature_column transformation
        if source_conf.is_tf_integrated:
            raise XGBoostError('So far, tf transformation is not supported in xgboost job.')

        self._train = source_conf.is_train
        self._rcd_builder = RecordBuilder(column_conf.features)

        # concatenate all column fields in use into `col_names`
        # in training mode, there exists [features, label, group(optional), weight(optional)]
        # in prediction mode, there exists [features, result_columns, additional_columns(optional)]
        spec_temp = {'is_sparse': False, 'shape': [1], 'delimiter': ''}
        metas = {}
        col_names = []
        for col in column_conf.features.columns:
            col_names.append(col)
            metas[col] = spec_temp
        self._feature_len = len(col_names)

        self._label_offset = -1
        self._group_offset = -1
        self._weight_offset = -1
        self._append_indices = []
        if self._train:
            # label, group, weight column are training specialized
            if column_conf.label:
                self._label_offset = len(col_names)
                col_names.append(column_conf.label)
                metas[column_conf.label] = spec_temp

            if column_conf.group:
                self._group_offset = len(col_names)
                col_names.append(column_conf.group)
                metas[column_conf.group] = spec_temp

            if column_conf.weight:
                self._weight_offset = len(col_names)
                col_names.append(column_conf.weight)
                metas[column_conf.weight] = spec_temp
        else:
            # append columns are prediction specialized
            for col in column_conf.append_columns:
                if col in metas:
                    self._append_indices.append(col_names.index(col))
                else:
                    self._append_indices.append(len(col_names))
                    col_names.append(col)
                    metas[col] = spec_temp

            self._result_schema = {'append_columns': column_conf.append_columns or []}
            self._result_schema.update(column_conf.result_columns._asdict())

        conn = connect(**source_conf.db_config)
        
        def writer_maker(table_schema):
            return buffered_db_writer(
                    driver=source_conf.db_config['driver'],
                    conn=conn,
                    table_name=source_conf.output_table,
                    table_schema=table_schema,
                    buff_size=source_conf.write_batch_size)

        self._writer_maker = writer_maker

        # Since label field has already been included into `col_names`, we just fill `label_column_name` with None.
        self._reader = db_generator(
            driver=source_conf.db_config['driver'],
            conn=conn,
            statement=source_conf.standard_select,
            feature_column_names=col_names,
            label_column_name=None,
            feature_specs=metas)

        if not self._train:
            if not source_conf.output_table:
                raise XGBoostError('Output_table must be defined in xgboost prediction job.')

    def _read_impl(self):
        label = None
        group = None
        weight = 1.0
        append_cols = []
        for columns, _ in self._reader():
            features = columns[:self._feature_len]
            if self._label_offset >= 0:
                label = columns[self._label_offset]
            if self._group_offset >= 0:
                group = columns[self._group_offset]
            if self._weight_offset >= 0:
                weight = columns[self._weight_offset]
            if self._append_indices:
                append_cols = [columns[i] for i in self._append_indices]
            yield features, label, group, weight, append_cols

    def read(self) -> Iterator[XGBoostRecord]:
        for features, label, group, weight, append_cols in self._read_impl():
            yield self._rcd_builder.build(features, label, group, weight, append_info=append_cols)

    def write(self, result_iter: Iterator[XGBoostResult]):
        # peek one record for schema inference
        peek_ret = result_iter.__next__()

        # build table_schema
        table_schema = [self._result_schema['result_column']]
        table_schema.extend(self._result_schema['append_columns'])
        output_leaf = False
        if peek_ret.leaf_indices and self._result_schema['leaf_column']:
            output_leaf = True
            table_schema.append(self._result_schema['leaf_column'])
        output_prob = False
        if peek_ret.classification_prob and self._result_schema['probability_column']:
            output_prob = True
            table_schema.append(self._result_schema['probability_column'])
        output_detail = False
        if peek_ret.classification_detail and self._result_schema['detail_column']:
            output_detail = True
            table_schema.append(self._result_schema['detail_column'])

        def make_row(xgb_ret: XGBoostResult):
            row = [xgb_ret.result]
            if xgb_ret.append_info:
                row.extend(xgb_ret.append_info)
            if output_leaf:
                row.append(','.join(map(str, xgb_ret.leaf_indices)))
            if output_prob:
                row.append(xgb_ret.classification_prob)
            if output_detail:
                detail = {i: p for i, p in enumerate(xgb_ret.classification_detail)}
                row.append(json.dumps(detail))
            return row 

        with self._writer_maker(table_schema) as w:
            w.write(make_row(peek_ret))
            for ret in result_iter:
                w.write(make_row(ret))
