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

# coding: utf-8
from __future__ import absolute_import, division, print_function

import warnings

import numpy as np
import pandas as pd
from sklearn import preprocessing

warnings.filterwarnings('ignore')

### Loading raw data
raw = pd.read_csv('household_power_consumption.csv')

### Selecting the field 'Global_active_power' for clustering
data = raw[['Date', 'Time', 'Global_active_power']]
data['Global_active_power'] = data['Global_active_power'].replace(
    '?', 0).astype(float)

### Data reconstruction
date = data['Date'].str.split('/', expand=True)[[0, 1]].astype('str')
dates = date[1] + '/' + date[0]
data['Date'] = dates

secs = data[data['Date'] == '1/1'].Time
days = data.Date.unique()

df_gap = pd.DataFrame([], columns=secs, index=days)
for i in days:
    df_gap.loc[i] = data[data['Date'] == i]['Global_active_power'].T.values

### Data aggregation
df_gap_agg = pd.DataFrame([], index=df_gap.index)
timegap = 30
for i in range(1440 // 30):
    df_gap_agg['m' + str(i + 1)] = df_gap.iloc[:, (i * timegap):(i + 1) *
                                               timegap].sum(1)

### Data scaling
df_gap_final = pd.DataFrame(
    preprocessing.MinMaxScaler().fit_transform(df_gap_agg),
    columns=df_gap_agg.columns,
    index=df_gap_agg.index)
df_gap_final.index.name = 'dates'
df_gap_final = df_gap_final.reset_index()
df_gap_final.to_csv('activepower.csv', index=False)
