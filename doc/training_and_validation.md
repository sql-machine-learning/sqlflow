# Design: Training and Validation

## Splitting

Validation is used to measure a trained model, like accuracy. In SQLFlow, automation is the core value. So SQLFlow proposed to do training and validation in one SQL. It requires spliting the user specified data into two parts: traning data and validation data correspond to two processes(train and validation) respectively.

## How to splitting

### Splitting the Training Table into Training Data and Validation Data

Follow the [discussion](https://github.com/sql-machine-learning/sqlflow/issues/390#issuecomment-497336262), we suppose SQLFlow are dealing with the following SQL to train an ML model:

```sql
​```
SELECT col1, col2, col3
FROM mytable
TRAN ...
​```
```

The data comes from the standard select part `SELECT col1, col2, col3 FROM mytable`, and let's say the query result looks like the following

| col1   | col2   | col3   |
| ------ | ------ | ------ |
| <data> | <data> | <data> |
| <data> | <data> | <data> |
| <data> | <data> | <data> |
| ...    |        |        |

We want to split the result into 80% training data and 20% validation data.

**First, we add a column train_val_split via RAND() and save the result to a temporary table.** Note the `RAND()` function returns a random number between 0 (inclusive) and 1. The result temporary table looks like the following.

| temp_table |        |        |                 |
| ---------- | ------ | ------ | --------------- |
| col1       | col2   | col3   | train_val_split |
| <data>     | <data> | <data> | 0.3             |
| <data>     | <data> | <data> | 0.9             |
| <data>     | <data> | <data> | 0.5             |
| ...        |        |        |                 |

We can generate the corresponding SQL using the following code template

```
CREATE TABLE {.TempTableName} AS (
    SELECT *, RAND() AS train_val_split FROM (
        {.StandardSQL}
    )
)
```

**Second, we fetch the training/validation data using two different queries respectively.** The query for training data can be written as `SELECT * FROM temp_table WHERE train_val_split <= 0.8`, which fetches row1 and row3 etc.. The query for validation data can be written as `SELECT * FROM temp_table WHERE train_val_split > 0.8`, which fetches the rest of the rows.



