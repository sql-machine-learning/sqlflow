# Design: Training and Validation

A common ML training job usually involves two kinds of datasets: training data and validation data. These two datasets will be generated automatically by SQLFlow through randomly splitting the select results.

## Overall
SQLFlow generates a temporary table following the user-specific dataset, trains and evaluates a model.

<img src="./figures/training_and_validation.png" width="60%">

Notice, we talk about the **train** process in this post.

## Generate a temporary table
Splitting the training table into training data and validation data is the key point. We suppose SQLFlow are dealing with the following SQL to train an ML model:

```SQL
SELECT col1, col2, col3
FROM mytable
TRAIN ...​
```

The data comes from the standard select part `SELECT col1, col2, col3 FROM mytable`, and let's say the query result looks like the following


| col1   | col2   | col3   |
| ------ | ------ | ------ |
| \<data\> | \<data\> | \<data\> |
| \<data\> | \<data\> | \<data\> |
| | ... | |

We want to split the result into 80% training data and 20% validation data.

### We add a column sqlflow_random via RAND() and save the result to a temporary table   
Note the `RAND()` function returns a random number between 0 (inclusive) and 1. The result temporary table looks like the following.

<table>
  <tr>
    <th colspan="4">temp_table</th>
  </tr>
  <tr>
    <td>col1</td>
    <td>col2</td>
    <td>col3</td>
    <td>sqlflow_random</td>
  </tr>
  <tr>
    <td>&lt;data&gt;</td>
    <td>&lt;data&gt;</td>
    <td>&lt;data&gt;</td>
    <td>0.350812148722645</td>
  </tr>
  <tr>
    <td>&lt;data&gt;</td>
    <td>&lt;data&gt;</td>
    <td>&lt;data&gt;</td>
    <td>0.9845468606655923</td>
  </tr>
  <tr>
    <td>&lt;data&gt;</td>
    <td>&lt;data&gt;</td>
    <td>&lt;data&gt;</td>
    <td>0.650812148722645</td>
  </tr>
  <tr>
    <td></td>
    <td>...</td>
    <td></td>
    <td></td>
  </tr>
</table>


We can generate the corresponding SQL using the following code template
```SQL
CREATE TABLE {.TempTableName} AS
    SELECT *, RAND() AS sqlflow_random FROM (
        {.StandardSQL}
    )
```

### Naming temporary table
- For common databases, like MySQL, Hive   
SQLFlow creates the temporary table with the attribute [TEMPORARY ](https://dev.mysql.com/doc/refman/8.0/en/create-temporary-table.html), so we do not have to test if the temporary table exists.

- For Maxcompute  
SQLFlow saves the temporary table into the current project which must be specified by the user. 
Meanwhile, we specify the [LIFECYCLE](http://help.aliyun-inc.com/internaldoc/detail/27808.html?spm=a2c1f.8259794.2.1.64a896d5dR0z6r) to 1 day to release the temporary table automatically.

  Because multi-users run SQLFlow with their own isolated dataset at the same time, SQLFlow generates an elaborate name for the temporary table to avoid conflict.

## How to split

**We fetch the training/validation data using two different queries respectively.** 
   
The query for training data can be written as `SELECT * FROM temp_table WHERE sqlflow_random < 0.8`, which fetches row1 and row3 etc.. The query for validation data can be written as `SELECT * FROM temp_table WHERE sqlflow_random >= 0.8`, which fetches the rest of the rows.

In SQLFlow, we modify the user-specific dataset to our temp_table restricted to `sqlflow_random >= 0.8` to train a model, then restricted the temp_table  to`sqlflow_random < 0.8` to validate that model. This context is built after the temporary dataset accomplished, passed to `runExtendedSQL`  in `extendedSelect`.

```Go
type extendedSelect struct {
    // ...
    training   standardSelect // training dataset
    validation standardSelect // validation dataset
}
```

## Codegen
For TensorFlow submitter, we generate training dataset and validation dataset according to `extendedSelect.training` and `extendedSelect.validation`.

## Release the temporary table
In the end, we remove the temporary table by
```SQL
drop table if exists {temporary_table_name}
```
We created the temporary table specified with `TEMPORARY` or `LIFECYCLE`, but SQLFlow would rather release the temporary table explicitly.

## Notes

- If the column sqlflow_random already exists, SQLFlow chooses to quit   
  Notice, *column name started with an underscore is invalid in the hive*
- Any discussion to implement a better splitting is welcomed