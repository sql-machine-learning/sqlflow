#!/bin/env python
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

import pandas as pd  # data processing, CSV file I/O (e.g. pd.read_csv)

train_df = pd.read_csv("./GiveMeSomeCredit/cs-training.csv", index_col=0)

train_df.MonthlyIncome.fillna(train_df.MonthlyIncome.median(), inplace=True)
train_df.NumberOfDependents.fillna(train_df.NumberOfDependents.median(),
                                   inplace=True)
train_df = train_df[(train_df['age'] > 0) & (train_df['age'] < 90)]

train_df[:2000].to_csv('/tmp/train.csv', index=False)
