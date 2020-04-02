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

import sys
import warnings

import numpy as np
import pandas as pd
import shap
from sklearn import preprocessing
from sklearn.model_selection import train_test_split

warnings.filterwarnings('ignore')


### Filling in numerical feature missing values
def fillna_num(df):
    numcols = df.dtypes[df.dtypes != 'object'].index
    for i in numcols:
        # Handling outliers. Reserved within 4sigma.
        miu = df[i].mean()
        sigma = np.std(df[i], ddof=1)
        lower = miu - 4 * sigma
        upper = miu + 4 * sigma
        df[(df[i] < lower) | df[i] > upper][i] = np.nan
        # According to the skewness to determine the filling situation,
        # if the absolute value of the skewness is greater than 2, fill in the median, otherwise fill in the mean
        if (np.abs(df[i].skew() > 2)):
            df[i].fillna(df.ix[df[i].isnull() == False][i].median(),
                         inplace=True)
        else:
            df[i].fillna(df.ix[df[i].isnull() == False][i].mean(),
                         inplace=True)
    return df


### Filling in category feature missing values
def fillna_obj(df):
    strcols = df.dtypes[df.dtypes == 'object'].index
    for i in strcols:
        df[i].fillna(df[i].mode()[0], inplace=True)
    return df


### Getting dummies for category features and scaling data
def dataPrepare(X, y):
    RANDOM_STATE = 20
    TEST_SIZE = 0.3
    numList = X.dtypes[X.dtypes != 'object'].index
    objList = X.dtypes[X.dtypes == 'object'].index
    if len(numList) > 0:
        X_num = pd.DataFrame(preprocessing.StandardScaler().fit_transform(
            X[numList]),
                             columns=numList)
        X_num.reset_index(drop=True, inplace=True)
    else:
        X_num = pd.DataFrame([])

    if len(objList) > 0:
        X_dummy = pd.concat(
            [pd.get_dummies(X[i], prefix=i, drop_first=True) for i in objList],
            axis=1)
        X_dummy.reset_index(drop=True, inplace=True)
    else:
        X_dummy = pd.DataFrame([])
    X_final = pd.concat([X_num, X_dummy], axis=1)
    Xtrain, Xtest, ytrain, ytest = train_test_split(
        X_final, y, test_size=TEST_SIZE,
        random_state=RANDOM_STATE)  #将数据集划分成训练集和测试集
    trainSet = pd.concat([Xtrain, ytrain], axis=1)
    testSet = pd.concat([Xtest, ytest], axis=1)
    return trainSet, testSet


### Loading data
df = pd.read_csv('data.csv')

### Data filtering
df = df.query('MSRP<50000').reset_index(drop=True)

### Constructing features
Market_Category = df['Market Category'].str.split(',')
Market_Category.fillna('', inplace=True)
df['Market Category_nums'] = [len(i) for i in Market_Category]
all_category = [
    'Factory Tuner', 'Luxury', 'High-Performance', 'Performance', 'Flex Fuel',
    'Hatchback', 'Hybrid', 'Diesel', 'Exotic', 'Crossover'
]
for category in all_category:
    temp = []
    for i in range(len(df)):
        temp.append(1) if category in Market_Category[i] else temp.append(0)

    df['category_' + category] = temp

### Filling the Missing data
df = fillna_obj(df)
df = fillna_num(df)

### Data conversion
df['Engine Fuel Type'] = df['Engine Fuel Type'].replace({
    'flex-fuel (premium unleaded required/E85)':
    'flex-fuel',
    'flex-fuel (premium unleaded recommended/E85)':
    'flex-fuel',
    'flex-fuel (unleaded/natural gas)':
    'flex-fuel',
    'natural gas':
    'flex-fuel',
    'flex-fuel (unleaded/E85)':
    'flex-fuel',
})

df.drop(columns=['Model', 'Market Category', 'Year'], inplace=True, axis=1)

df['MSRP'] = (df['MSRP'] - df['MSRP'].mean()) / df['MSRP'].std()

label = 'MSRP'
X = df.drop(label, axis=1)
y = df[label]
trainSet, testSet = dataPrepare(X, y)

trainSet.columns = [
    i.replace(' ', '_').replace('-', '_').replace('(', '').replace(')',
                                                                   '').lower()
    for i in trainSet.columns
]
testSet.columns = [
    i.replace(' ', '_').replace('-', '_').replace('(', '').replace(')',
                                                                   '').lower()
    for i in testSet.columns
]

trainSet.to_csv('trainSet.csv', index=False)
testSet.to_csv('testSet.csv', index=False)
