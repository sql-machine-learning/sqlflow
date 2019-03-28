# Data Partitioning

Deep learning systems usually use GPUs to accelerate computing.  GPUs are so efficient that the bottleneck is often the I/O bandwidth.  Given that said, when we do distributed training, we want every worker reads from the data source, other than that the master reads data and dispatches to workers.  To make sure that data read by different workers are disjoint, we need to partition the data and let workers read different partitions.

## Static and Dynamic Partitioning

Some training systems partition the data before training.  For example, some MPI-based systems require data partitioned into N files, where N is a fixed number of workers to be started in a training job.  We refer to this kind of partition as *static partitioning*.

If we want to do elastic scheduling of training jobs, the number of workers, N, might change at runtime.  A standard solution is to have N partitions, where N is much larger than the number of workers, and to dispatch partitions to workers at runtime.

## Data in Files and Databases

To make it easy to partition data in files, we could use a specially designed [data format](https://github.com/wangkuiyi/recordio), which allows the master to build an index of data instances by quickly scanning over the file using a lot of fseek calls.  Given the index, it would be easy to partition the data instances in the file into the M partitions.

However, SQLFlow's training data don't come from files, but results of SELECT statements.  In this article, we design a data partitioning mechanism that represents each partition by a SELECT statement derived from the original one but with appended WHERE clauses.

### Data Statistics

It is noticeable that machine learning training algorithms take *features* other than *data* as their inputs.  To convert data into features, we need data statistics, which comes from scanning at least part of the data.  For example, the TensorFlow feature column API, which is a neat tool to convert data into features, requires a vocabulary of the values of a field before it can transform the field into a [*categorical feature column with vocabulary list*](https://www.tensorflow.org/api_docs/python/tf/feature_column/categorical_column_with_vocabulary_list).  Another example is that the [*bucketized feature column*](https://www.tensorflow.org/api_docs/python/tf/feature_column/bucketized_column) requires a histogram of possible values of a field.

To build such statistics from a down-sample of the data, we could add a `LIMIT` clause to the original SELECT statement.   Say, the original one is `SELECT name, gender FROM employees`, the derived one should be `SELECT name, gender FROM employees LIMIT 10000`, where 10000 is a value that we can handle in a reasonably short time.

### Indexing and Partitioning

Given the statistics, or histogram of the joint distribution of the data, we could make even partitions.  Suppose there is a table with two fields: name and gender, and 20,000 rows.  Possible values of gender are male, female, and unknown.  There is an infinite number of possible names.  All possible rows distribute in a two-dimensional space, and a possible partitioning is as follows:


```
         |  Aaron | Aby | ....
-----------------------------------
male     |              |             |
female   |  partition 1 | partition 2 | ...
unknown  |              |             |
```

An alternative partitioning is not along the alphabetic order of names but to hash the names so that each partition includes names with different prefixes.

Each partition is represented by a SQL statemnt, for example:

- `SELECT name, gender FROM employees WHERE hash(name) + hash(gender) % 100 = 0`
- `SELECT name, gender FROM employees WHERE hash(name) + hash(gender) % 100 = 1`
- `SELECT name, gender FROM employees WHERE hash(name) + hash(gender) % 100 = 2`

where 100 is the total number of partitions.

### The Number of Partitions

A quick-and-simple estimate of M is to divide the total number of rows, R, by the preferred partition size.  To get R, we can use the SQL function COUNT.  In particular, we can replace the field names in the original statement by a call `COUNT(*)`.  For example, change the original statement `SELECT name, gender FROM employees` into `SELECT COUNT(*) FROM employees`.

A challenge here is that SQLFlow's SQL parser has to be able to provide sufficient information to make the replacement possible.  Currently, it cannot.
