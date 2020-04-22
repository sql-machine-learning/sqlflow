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

import numpy as np
import pandas as pd
import sklearn.preprocessing as preprocessing
from sklearn.ensemble import RandomForestRegressor

### Loading data
train = pd.read_csv('train.csv')
test = pd.read_csv('test.csv')

### Concat train + test
alldata = pd.concat(
    [train.ix[:, 'Pclass':'Embarked'],
     test.ix[:, 'Pclass':'Embarked']]).reset_index(drop=True)

### Cleaning the data
# Fare
alldata['Fare'] = alldata['Fare'].fillna(alldata['Fare'].mean())  # median()

# Embarked
alldata['Embarked'] = alldata['Embarked'].fillna(alldata['Embarked'].mode()[0])


# Age: Fill in missing values with RandomForestClassifier
def set_missing_ages(df):
    age_df = df[['Age', 'Fare', 'Parch', 'SibSp', 'Pclass']]
    known_age = age_df[age_df.Age.notnull()].as_matrix()
    unknown_age = age_df[age_df.Age.isnull()].as_matrix()
    y = known_age[:, 0]
    X = known_age[:, 1:]
    rfr = RandomForestRegressor(random_state=10, n_estimators=2000, n_jobs=-1)
    rfr.fit(X, y)
    predictedAges = rfr.predict(unknown_age[:, 1::])
    df.loc[(df.Age.isnull()), 'Age'] = predictedAges
    return df, rfr


alldata, rfr = set_missing_ages(alldata)

### Constructing features
alldata['CabinHead'] = alldata['Cabin'].str[0]
alldata['CabinHead'] = alldata['CabinHead'].fillna('None')
alldata['CabinAlpha'] = (alldata['CabinHead'].isin(['B', 'D', 'E'])) * 1
alldata['NullCabin'] = (alldata['Cabin'].notnull() == True) * 1
alldata['NullCabin'] = alldata['NullCabin'].fillna(0)
alldata['NoSibSp'] = (alldata['SibSp'] <= 0) * 1
alldata['NoParch'] = (alldata['Parch'] <= 0) * 1
alldata['Family'] = alldata['SibSp'] + alldata['Parch'] + 1
alldata['isAlone'] = (alldata['Family'] == 1) * 1

# Constructing a real fare for everyone from Ticket
Ticket = pd.DataFrame(alldata['Ticket'].value_counts())
Ticket.columns = ['PN']
Ticket.head()
alldata1 = pd.merge(alldata, Ticket, left_on='Ticket', right_index=True)
alldata['realFare'] = alldata['Fare'] / alldata1['PN']

# Constructing each person's rank from Name
alldata['Title'] = alldata['Name'].str.split(", |\.", expand=True)[1]
alldata.ix[alldata['Title'].isin(['Ms', 'Mlle']), 'Title'] = 'Miss'
alldata.ix[alldata['Title'].isin(['Mme']), 'Title'] = 'Mrs'
stat_min = 10
title_names = (alldata['Title'].value_counts() < stat_min)
alldata['Title'] = alldata['Title'].apply(lambda x: 'Misc'
                                          if title_names.loc[x] == True else x)

# Constructing a ismother label
alldata['ismother'] = ((alldata['Sex']=='female') & (alldata['Parch'] > 0) \
                    & (alldata['Age']>=16) & (alldata['Title']=='Mrs')) *1

alldata = alldata.drop(
    ['Name', 'SibSp', 'Parch', 'Ticket', 'Fare', 'Cabin', 'CabinHead'], axis=1)

### Dividing the preprocessed files into trainSet and testSet
train_ = pd.concat([alldata.iloc[:train.shape[0], :], train[['Survived']]],
                   axis=1)
test_ = alldata.iloc[train.shape[0]:, :]

### Getting dummies for category features
objList = ['Pclass', 'Sex', 'Embarked', 'Title']
temp_obj = pd.concat([pd.get_dummies(train_[i], prefix=i) for i in objList],
                     axis=1)
temp_num = train_[[
    'NoSibSp', 'NoParch', 'NullCabin', 'CabinAlpha', 'Family', 'isAlone',
    'ismother', 'Age', 'realFare', 'Survived'
]]
trainSet = pd.concat([temp_obj, temp_num], axis=1)

temp_obj = pd.concat([pd.get_dummies(test_[i], prefix=i) for i in objList],
                     axis=1)
temp_num = test_[[
    'NoSibSp', 'NoParch', 'NullCabin', 'CabinAlpha', 'Family', 'isAlone',
    'ismother', 'Age', 'realFare'
]]
testSet = pd.concat([temp_obj, temp_num], axis=1)

### Scaling data
scaler = preprocessing.StandardScaler()
age_scale_param = scaler.fit(trainSet[['Age']])
trainSet['Age'] = scaler.fit_transform(trainSet[['Age']], age_scale_param)
testSet['Age'] = scaler.fit_transform(testSet[['Age']], age_scale_param)

fare_scale_param = scaler.fit(trainSet[['realFare']])
trainSet['realFare'] = scaler.fit_transform(trainSet[['realFare']],
                                            fare_scale_param)
testSet['realFare'] = scaler.fit_transform(testSet[['realFare']],
                                           age_scale_param)

trainSet.columns = [i.lower() for i in trainSet.columns]
testSet.columns = [i.lower() for i in testSet.columns]

trainSet.to_csv('train_dp.csv', index=False)
testSet.to_csv('test_dp.csv', index=False)
