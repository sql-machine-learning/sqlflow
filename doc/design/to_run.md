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
clause in the `TO TRAIN` clause, and they can write standard statements like
`SELECT .. INTO new_table` to pre-process the data.

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
runs a sliding-window over the time-series data, convert each step into a new
row and then calculate many statistaical values on it to derive features.

This requirement intrigues us to support a general way to call arbitrary
programs from SQL, hence the idea of the `TO RUN` clause.

## Syntax Extension

We want the syntax extension support to run any program written in any language,
and we also want it easy to run Python programs in a way that best fits the
current SQLFlow deployment.  Let us dive deep into the no-compromise design.

### Run Any Program

The subject to run is a program.  A program needs versioning and releasing.  As
always, we assume Docker images are released form.  Therefore, the user needs to
provide a Docker image as the subject of the `TO RUN` clause.  The SQLFlow
compiler translates the `TO RUN` statement into a Tekton step that runs this
Docker image, or more specificially, the entrypoint program of the Docker image.

The following is an example to run an executable built from a program written
in Go/C++/.etc.

```SQL
SELECT * FROM source_table ORDER BY creation_date
TO RUN a_data_scientist/ts_data_processor:1.0
CMD
  "slide_window_to_row",
  "--time_column=t",
  "--value_column=v",
  "--window_width=120"
INTO output_table_name;
```

The SQLFlow compiler translates it into a Tekton step that
executes the Docker image `a_data_scientist/ts_data_processor:1.0` with the
command-line options

- `"slide_window_to_row"`
- `"--time_column=t"`
- `"--value_column=v"`
- `"--window_width=120"`

and environment variables

- `SQLFLOW_TO_RUN_SELECT=SELECT * FROM source_table ORDER BY creation_date`
- `SQLFLOW_TO_RUN_INTO=output_table_name`
- `SQLFLOW_TO_RUN_IMAGE=a_data_scientist/ts_data_processor:1.0`

We will talk more about the command-line options and environment variables
later in this document.

The following is an example to run a program written in a script language
such as Python.

```SQL
SELECT * FROM source_table ORDER BY creation_date
TO RUN a_data_scientist/extract_ts_features:1.0
CMD
  "ts_feature_extractor.py",
  "--time_column=t",
  "--value_column=x",
  "--window_width=120"
INTO output_table_name;
```

### The SELECT Prefix

The semantics of `SELECT input_table TO RUN function_image CMD parameters INTO output_table`
is that retrieve the data from `input_table`, process the data using the
executable in the `function_image` with `parameters` and then output the
result into `output_table`.  
As the `TO RUN` clause can run any program in the Docker image, it is not
necessary to have an input table.  From the SQL users' point of view, it's
more user friendly to keep the syntax SQL style.  As a result, we always
keep the SELECT prefix in `TO RUN` statement.  If there is no input table,
user can write `SELECT 1` as a prefix.

```SQL
SELECT 1
TO RUN a_data_scientist/ts_data_processor:1.0
CMD
  "process_without_input_table",
  "--time_column=t",
  "--value_column=v",
  "--window_width=120"
INTO output_table_name;
```

### The INTO Suffix

Just mentioned in the last section, `INTO` represents the output table of `TO RUN`.
The `TO RUN` clause can run any program in the Docker image. The program can
output more than one table or don't output any table. Please check the following
examples.

Output two tables - multiple table names after `INTO`:

```SQL
SELECT * FROM source_table ORDER BY creation_date
TO RUN a_data_scientist/ts_data_processor:1.0
CMD
  "process_with_two_output_tables",
  "--time_column=t",
  "--value_column=v",
  "--window_width=120"
INTO output_table_1, output_table_2;
```

No output table - no `INTO` keyword:

```SQL
SELECT * FROM source_table ORDER BY creation_date
TO RUN a_data_scientist/ts_data_processor:1.0
CMD
  "process_without_output_table",
  "--time_column=t",
  "--value_column=v",
  "--window_width=120";
```

## The Context

In the above example, the entrypoint program takes four command-line options:
`slide_window_to_row`, `--time_column=t`, `--value_column=v`, `--window_width=120`.
Also, the program needs context information, including the DBMS endpoints,
credential information to access the data, and the SELECT prefix.

The SQLFlow compiler has to pass context in the form of environment variables
other than command-line options because some command-line parsing frameworks
terminates the program seeing unknown options.  When SQLFlow upgrades and
introduces new options, the entrypoint program would fail.

The SQLFlow server cannot pass in all context information in a single
environment variable, which has a limit of value size.  Instead, it sets
environment variables prefixed with `SQLFLOW_`.

- `SQLFLOW_TO_RUN_SELECT`
- `SQLFLOW_TO_RUN_INTO`
- `SQLFLOW_TO_RUN_IMAGE`
- `SQLFLOW_DB_TYPE`: the type of DBMS.
- `SQLFLOW_DEPLOYMENT_PLATFORM`: Kubernetes | MaxCompute | GoogleCloud | ...

## Run a Python Program

Just as the beginning of this article, the original intention of `TO RUN`
statement is data transformation in end-to-end machine learning.  Besides
SQL, data scientists usually write Python program to process the data.
And there are quite a few mature python packages to leverage for data
processing such as numpy, pandas, sklearn, etc.  We are focusing on how to run
a Python program in `TO RUN` statement in this section.

The subject of `TO RUN` is a docker image. The author provides an executable
built from any language. For Python, it's a complete Python program.  Since
it can accept the command line parameters from `TO RUN` statement, the program
need a `main` function, parse the arguments and then execute with these args.
Because Python program has dependencies, the author needs to provide a
Dockerfile.  They can use a standard base image that contains the standard
entrypoint program.  The base image could be defined as follows.

```dockerfile
FROM ubuntu:18.04
COPY . /src
ENV PYTHONPATH /src/python_eval.py
ENTRYPOINT ["/src/python_eval.py"]
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
TO RUN a_data_scientist/my_python_zoo
CMD
  "a_python_func(parameters)",
  "another_python_func(params)";
```

## Distributed Data Processing

The above abstraction enables `TO RUN` to execute a Python program locally in a
Tekton step container.  This function can call Kubernetes API to start some
jobs.  For example, it can launch a Dask job on Kubernetes to have multiple
workers running the same Python function to pre-process the data in parallel.

## Execution Platforms
