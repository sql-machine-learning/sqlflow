# SQLFlow Syntax Extension: `TO RUN`


## Data Transformation

As the purpose of SQLFlow is to extend SQL syntax for end-to-end machine
learning, we have to consider *data transformation*.  Usually, there are two
kinds of data transformations:

1. *pre-processing*, which happens before the training stage. Examples include
   joining a feature lookup table with the training data table.
1. *in-train loop transformation*, which happens in the training loop for each
   data instance.

The in-train loop transformation differs from the pre-processing primarily in
that they are part of the trained models and used in predictions.

SQLFlow users can describe the in-train loop transformation using the `COLUMN`
clause in the `TO TRAIN`-clause, and they can write standard statements like
`SELECT .. INTO new_table` to pre-process data.

This design is about pre-processing that we cannot express using standard SQL
statements.


## The Challenge

SQL is extensible and thus powerful.  Most DBMS support user-defined functions
(UDFs).  The SELECT statement applies UDFs and built-in functions to each row of
a table/view.  However, some pre-processing transformations are not row-wise.
An example is *sliding-window*.

A [user told us](https://github.com/sql-machine-learning/sqlflow/issues/2238)
that he wants to extract temporal features from time-series data by calling the
well-known Python package [`tsfresh`](https://tsfresh.readthedocs.io), which
runs a sliding-window over the time-series data and convert each step into a new
row.

This requirement intrigues us to support a general way to call arbitrary
programs from SQL, hence the idea of the `TO RUN` clause.


## Syntax Extension

The subject to run is a program.  A program needs versioning and releasing.  As
always, we assume Docker images are released form.  Therefore, the user needs to
provide a Docker image as the subject of the `TO RUN` clause.  The SQLFlow
compiler translates the TO RUN statement into a Tekton step that runs this
Docker image.

```sql
SELECT * FROM source_table ORDER BY creation_date
TO RUN 
  a_data_scientist/extract_ts_features:1.0
CMD 
  "--verbose",
  "--window_width=120"
INTO output_table_name;
```

In this example, the entrypoint program is likely written in Python so it can
call the `tsfresh` Python package.  The CMD clause provides command-line options
to the entrypoint program.


### The SELECT Prefix

As the `TO RUN` clause can run any program in the Docker image, it is not
necessary to have the SELECT statement as a prefix.  Instead, we can change the
above syntax design to make the SELECT prefix a command-line option to the
entrypoint program, which then calls the DBMS API to execute it.

```sql
TO RUN 
  a_data_scientist/extract_ts_features:1.0
CMD 
  "SELECT * FROM source_table ORDER BY creation_date"
  "--verbose",
  "--window_width=120"
  "output_table_name";
```

However, we still prefer the SELECT statement as a prefix, but the SQLFlow
compiler doesn't run it as a step container; instead, the compiler passes the
SELECT statement to the entrypoint program as part of the context.

### The INTO Suffix

Like the way it handles the SELECT prefix, the compiler passes the INTO suffix
to the entrypoint program as part of the context.

## The Context

In the above example, the entrypoint program takes three command-line options:
`--verbose`, `--window_width=120`, and `output_table_name`.  Also, the program
needs context information, including the DBMS endpoints, credential information
to access the data, and the SELECT prefix.

The SQLFlow compiler has to pass context in the form of environment variables
other than command-line options because some command-line parsing frameworks
terminates the program seeing unknown options.  When SQLFlow upgrades and
introduces new options, the entrypoint program would fail.

The SQLFlow server cannot pass in all context information in a single
environment variable, which has a limit of value size.  Instead, it sets
environment variables prefixed with `SQLFLOW_`.

- `SQLFLOW_RUN_SELECT`
- `SQLFLOW_RUN_INTO`
- `SQLFLOW_DB`: the type of DBMS.
- [To-be-complete]


## Run a Python Function

Some contributors might want to simply provide a Python function call to the `TO
RUN` clauses.  In such cases, we need a standard entrypoint program that
evaluates the Python function call.

Because Python functions have dependencies, the author needs to provide a
Dockerfile. They can use a standard base image that contains the standard
entrypoint program.  The base image could be defined as follows to include the
Python function call evaluator.

```dockerfile
FROM ubuntu:18.04
COPY . /src
ENV PYTHONPATH /src/python_eval.py
ENTRYPOINT ["/src/python_eval.py"]
```

An over-simplified implementation of `python_eval.py` evaluates its command-line
options one-by-one.

```python
import sys
if __name__ == "__main__":
    for i, arg in enumerate(sys.argv):
        if i > 0:
            eval(arg)
```

Given the above base Docker image, say, `sqlflow/run:python`, contributors can
derive their images by adding their Python code.

```dockerfile
FROM sqlflow/run:python 
COPY . /opt/python
ENV PYTHONPATH /opt/python
```

Suppose that the above Dockerfile builds into image
`a_data_scientist/my_python_zoo`, SQLFlow users can run it with the following
statement.

```sql
SELECT ... 
TO RUN
  a_data_scientist/my_python_zoo
CMD
  "a_python_func(parameters)",
  "another_python_func(params)";
```


## Distributed Data Pre-processing

The above abstraction enables TO RUN to execute a Python function locally in a
Tekton step container.  This function can call Kubernetes API to start some
jobs.  For example, it can launch a Dask job on Kubernetes to have multiple
workers running the same Python function to pre-process the data in parallel.
