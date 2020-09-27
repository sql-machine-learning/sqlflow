# Credit ScoreCard Model on SQLFlow

The credit scorecard model is a common model in the financial lending scenario. A scorecard model outputs a score representing
how likely the lender will repay on time. This tutorial introduces how to train a credit scorecard model with SQLFlow.

## Dataset Introduction

This tutorial using a public dataset [Give Me Some Credit](https://www.kaggle.com/c/GiveMeSomeCredit/data)
on the [Kaggle](https://www.kaggle.com) website. The dataset contains 150,000 rows training data,
and each row includes 11 columns, where `serious_dlqin2yrs` is the target column and represents person experienced 90 days
past due delinquency or worse. The description for each column is as the table:

| Column                                      | Description                                                  | Type        |
| ------------------------------------------- | ------------------------------------------------------------ | --------------- |
| serious_dlqin2yrs                           | Person experienced 90 days past due delinquency or worse     | 1/0 |
| revolving_utilization_of_unsecured_lines    | Total balance on credit cards and personal lines of credit except for real estate and no installment debt like car loans divided by the sum of credit limits | Float|
| age                                         | Age of borrower in years| Integer |
| number_of_time30_59days_past_due_not_worse  | Number of times borrower has been 30-59 days past due but no worse in the last two years.| Integer |
| debt_ratio                                  | Monthly debt payments, alimony, living costs divided by monthly gross income| Float          |
| monthly_income                              | Monthly income| Integer|
| number_of_open_credit_lines_and_loans       | Number of Open loans (installment like car loan or mortgage) and Lines of credit (e.g. credit cards) | Integer |
| number_of_times_90_days_late                | Number of times borrower has been 90 days or more past due| Integer |
| number_real_estate_loans_or_lines           | Number of mortgage and real estate loans including home equity lines of credit| Integer |
| number_of_time60_89_days_past_due_not_worse | Number of times borrower has been 60-89 days past due but no worse in the last two years| Integer |
| number_of_dependents                        | Number of dependents in family excluding themselves (spouse, children, etc.)| Integer|

## Dataset Loading and Preprocessing

Please note, you can skip this section and use the preprocessed dataset inner SQLFlow playground.

Download and extract the zip file from the [download page](https://www.kaggle.com/c/GiveMeSomeCredit/data?select=cs-training.csv),
and execute the Python script [give_me_some_credit.py](/doc/tutorial/scripts/give_me_some_credit.py) to finish the data preprocess.
The Python scripts would output a CSV file `train.csv`, and you can run the following SQL program to
popularize the training table `scorecard.train`.

``` sql
DROP DATABASE IF EXISTS scorecard;
CREATE DATABASE scorecard;
DROP TABLE IF EXISTS scorecard.train;
CREATE TABLE IF NOT EXISTS scorecard.train(
  serious_dlqin2yrs int,
  revolving_utilization_of_unsecured_lines float,
  age int,
  number_of_time30_59days_past_due_not_worse int,
  debt_ratio float,
  monthly_income float NULL,
  number_of_open_credit_lines_and_loans int,
  number_of_times_90_days_late int,
  number_real_estate_loans_or_lines int,
  number_of_time60_89_days_past_due_not_worse int,
  number_of_dependents float
);
-- load train and test data from CSV files
LOAD DATA LOCAL INFILE '/tmp/train.csv' INTO TABLE scorecard.train FIELDS TERMINATED BY ',' ENCLOSED BY '"' LINES TERMINATED BY '\n' IGNORE 1 ROWS;
```

## Credit ScoreCard Modeling

You can have a glance at the training data by running the following SQL.

```sql
%%sqlflow
SELECT * FROM scorecard.train LIMIT 10;
```

Run the following command to start the credit scorecard modeling.

```sql
SELECT * FROM scorecard.train
TO TRAIN sqlflow_models.ScoreCard
LABEL serious_dlqin2yrs
INTO sqlflow_models.my_scorecard_model;
```

The above `TRAIN` clause would output the scorecard as the following. The final total score
is the sum of all scores based on the independent variable's value. The target score
is **600**, meaning that a user with a score higher than 600 will grant the credit.

``` text
 TARGET SCORE: 600
 age (20.999, 38.0] 106.0
 age (38.0, 47.0] 108.0
 age (47.0, 56.0] 101.0
 age (56.0, 65.0] 99.0
 age (65.0, 89.0] 93.0
 debt_ratio (-0.001, 0.132] 87.0
 debt_ratio (0.132, 0.287] 103.0
 debt_ratio (0.287, 0.472] 93.0
 debt_ratio (0.472, 3.248] 118.0
 debt_ratio (3.248, 15466.0] 101.0
 monthly_income (-0.001, 3332.4] 106.0
 monthly_income (3332.4, 5212.6] 108.0
 monthly_income (5212.6, 5400.0] 100.0
 monthly_income (5400.0, 8180.0] 100.0
 monthly_income (8180.0, 208333.0] 98.0
 number_of_dependents (-0.001, 2.0] 103.0
 number_of_dependents (2.0, 8.0] 98.0
 number_of_open_credit_lines_and_loans (-0.001, 4.0] 108.0
 number_of_open_credit_lines_and_loans (4.0, 6.0] 101.0
 number_of_open_credit_lines_and_loans (6.0, 9.0] 98.0
 number_of_open_credit_lines_and_loans (9.0, 12.0] 97.0
 number_of_open_credit_lines_and_loans (12.0, 46.0] 108.0
 number_of_time30_59days_past_due_not_worse (-0.001, 98.0] 103.0
 number_of_time60_89_days_past_due_not_worse (-0.001, 98.0] 103.0
 number_of_times_90_days_late (-0.001, 98.0] 103.0
 number_real_estate_loans_or_lines (-0.001, 1.0] 101.0
 number_real_estate_loans_or_lines (1.0, 2.0] 102.0
 number_real_estate_loans_or_lines (2.0, 9.0] 119.0
 revolving_utilization_of_unsecured_lines (-0.001, 0.0191] 74.0
 revolving_utilization_of_unsecured_lines (0.0191, 0.0867] 66.0
 revolving_utilization_of_unsecured_lines (0.0867, 0.276] 70.0
 revolving_utilization_of_unsecured_lines (0.276, 0.686] 94.0
 revolving_utilization_of_unsecured_lines (0.686, 2340.0] 139.0
```
