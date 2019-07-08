# Design: Training and Validation

Validation is used to evaluate the trained model, like accuracy rating, overfitting, etc. In SQLFlow, automation is the core value. So SQLFlow proposed to do training and validation in one SQL. It requires splitting the user-specific data into two parts: training data and validation data correspond to two processes(train and validation) respectively. 

Notice, we talk about the **train** process in this post.

## Overall

SQLFlow generates a temporary table following the user-specific dataset, trains and evaluates a model.

<img src="/Users/weiguo/go/src/github.com/sql-machine-learning/sqlflow/doc/figures/training_and_validation.png" width="60%">

## Generate a temporary dataset

Splitting the training table into training data and validation data is the key point. We suppose SQLFlow are dealing with the following SQL to train an ML model:

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

**We add a column sqlflow_random via RAND() and save the result to a temporary table.** Note the `RAND()` function returns a random number between 0 (inclusive) and 1. The result temporary table looks like the following.

| temp_table |        |        |                |
| ---------- | ------ | ------ | -------------- |
| col1       | col2   | col3   | sqlflow_random |
| <data>     | <data> | <data> | 0.3            |
| <data>     | <data> | <data> | 0.9            |
| <data>     | <data> | <data> | 0.5            |
| ...        |        |        |                |

We can generate the corresponding SQL using the following code template

```
CREATE TABLE {.TempTableName} AS
    SELECT *, RAND() AS sqlflow_random FROM (
        {.StandardSQL}
    )
```

## How to split

1. Split the temporary dataset

   **We fetch the training/validation data using two different queries respectively.** The query for training data can be written as `SELECT * FROM temp_table WHERE sqlflow_random < 0.8`, which fetches row1 and row3 etc.. The query for validation data can be written as `SELECT * FROM temp_table WHERE sqlflow_random >= 0.8`, which fetches the rest of the rows.

   In SQLFlow, we modify the user-specific dataset to our temp_table restricted to `sqlflow_random >= 0.8` to train a model, then restricted the temp_table  to`sqlflow_random < 0.8` to validate that model. This context is built after the temporary dataset accomplished, passed to `runExtendedSQL`  in `extendedSelect`.

   ```go
   type extendedSelect struct {
     // ...
     training   standardSelect // training dataset
     validation standardSelect // validation dataset
   }
   ```

2. Split `runExtendedSQL` to `train & validate` on training

```go
func runExtendedSQL(slct string, db *DB, pr *extendedSelect, modelDir string) *PipeReader { 
  // ...
  if os.Getenv("SQLFLOW_submitter") == "alps" {
    pr = split
    return submitALPS(wr, pr, db, cwd) // training & validation in a same function
  }
  
  if pr.train {
    train(pr, slct, db, cwd, wr, modelDir)           // using pr.training
    return validate(pr, slct, db, cwd, wr, modelDir) // using pr.validation
  }
  // ...
}
```

`validate` returns the evaluation of the trained model.

## Codegen

For tensorflow submitter, we generate `evaluation` code to evaluate the model. 

## Notes

- If the temporary table exists, SQLFlow chooses to quit rather than drop&re-create the temporary table.

  Or, the training process is not completed.

  Similarly, the column `sqlflow_random` already exists. (Notice, a column name started with an underscore is invalid in the hive)

- Any discussion to implement a better splitting is welcomed.