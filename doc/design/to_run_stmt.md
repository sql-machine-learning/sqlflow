# To Run Statement

## The Problem

SQLFlow extends the SQL syntax and can describe an end-to-end machine learning pipeline. Data transformation is an important part in the entire pipeline. Currently we have the following two options for data transformation:

- SQL.
- COLUMN clause for per data instance transform. The transform logic is saved into the model.

But that's not enough. It's a common case that the transformation is not per data instance which COLUMN clause is not suitable for. And we may need write very complex SQL or even cannot use SQL to express the transfomration logic. Let's take `extract_features` from [tsfresh package](https://tsfresh.readthedocs.io/en/latest/api/tsfresh.feature_extraction.html#module-tsfresh.feature_extraction.extraction) for example. It will preprocess the time series data, calculate statistical values using various analysis functions and parameters, and then generate tens or hunderds of features. [Convert the time series data of sliding window to rows](https://github.com/sql-machine-learning/sqlflow/issues/2238) is also a typical scenario.

We can express the data transform logic using Python, leverage many mature python packages and encapsulate it into a reusable function. And then we propose to add the `TO RUN` clause in SQLFlow to call it.

## Syntax Extension

```SQL
SELECT * FROM source_table
TO RUN a_data_scientist/maxcompute_functions:1.0/my_func.data_preprocessor.a_python_func
WITH param_a = value_a,
     param_b = value_b
INTO result_table
```

The SQLFlow statement above will call the python function `a_python_func` from the file `/run/my_func/data_preprocessor.py` inside the docker image `a_data_scientist/maxcompute_functions:1.0`. The attributes in the `WITH` clause will be passed to the python function as parameters.

## Challenges

- TO RUN can execute functions in Kubernetes and MaxCompute.
- TO RUN can execute functions in both a single process and a distributed cluster.
- TO RUN can execute functions built upon various data computing frameworks such as [Dask](https://github.com/dask/dask), [Mars](https://github.com/mars-project/mars), [Ray](https://github.com/ray-project/ray) and so on.

## Design

### Address the challenges

The semantics of `TO RUN a_data_scientist/maxcompute_functions:1.0/my_func.data_preprocessor.a_python_func` is that `a_python_func` will be executed in a docker container for Kubernetes or a PyODPS node for MaxCompute. The implementation of `a_python_func` is fully customized by users. Because the execution environments for `a_python_func` are different between Kubernetes and MaxCompute, there are some differences considering these three challenges above.

Kubernetes

- Execution environment  
  `a_python_func` is executed inside the function image `a_data_scientist/maxcompute_functions:1.0`. We can customized this image at the `docker build` stage and make sure that all the dependencies `a_python_func` need are already installed in the image.

- Single process execution  
  User writes the data transform process using pandas, numpy, tsfresh and so on inside `a_python_func`.

- Distributed cluster execution  
  User can install Dask/Mars/Ray while building the docker image. Inside the implementation of `a_python_func`, we can create a Dask/Mars/Ray cluster, build a computing graph and submit the distribution execution using the API from these packages.

MaxCompute

- Execution environment  
  The packages of PyODPS node are already pre-installed and user cannot customize it. `a_python_func` can only use these pre-installed python packages.

- Single process execution
  Since PyODPS node cannot guarantee it contains all the packages we need such as `tsfresh`, `a_python_func` create a mars cluster containing only one worker and tell this worker to execute another function `another_python_func` to do the data transformation via [Mars remote API](https://github.com/mars-project/mars/issues/1227). The cluster uses our function image `a_data_scientist/maxcompute_functions:1.0`, `another_python_func` and its dependencies are already installed while building this image.
  
- Distributed cluster execution  
  The PyODPS node only installed Mars package, but not Dask and Ray. So we can create a cluster, build the computing graph and submit the distributed execution only using Mars API inside `a_python_func`.

### How to build TO RUN function

#### Kubernetes

```TXT
-- my_func
---- data_preprocessor.py
---- utils.py (optional)
-- Dockerfile
```

`a_python_func` is implemented in `data_preprocessor.py`. `docker build -f Dockerfile .` will install the dependency and copy `my_func` folder into `/run/my_func` in the image. `utils.py` contains some helper utils to be used in `data_preprocessor.py`.

#### MaxCompute

```TXT
-- my_func
---- data_preprocessor.py
---- utils.py (optional)
-- Dockerfile
```

Build a computing graph using Mars API and submit into mars cluster. `data_preprocessor.py` contains only one function `a_python_func`.

```Python
def a_python_func(param_a, param_b):
    import mars.dataframe as md
    import mars.tensor as mt

    # Create a mars cluster containing 2 workers
    client = o.create_mars_cluster(2, 4, 16, min_worker_num=1)
    result = md.DataFrame(mt.random.rand(1000, 3)).sum().execute()
    client.stop_server()
```

Tell the worker to execute the data transformation the dependency of which is not included in PyODPS node but in function image. `data_preprocessor.py` contains two functions `a_python_func` (execute in PyODPS node) and `another_python_func` (execute in the mars worker pod).

```Python
def a_python_func(param_a, param_b):
    import mars.remote as mr

    def func():
        import sys
        sys.path.append('/run/my_func')

        from data_preprocessor import another_python_func
        another_python_func(param_a, param_b)

    # Create a mars cluster containing 1 worker
    client = o.create_mars_cluster(1, 4, 16)
    # Use mars remote api to tell the worker execute func.
    mr.spawn(func, args=()).execute()

    client.stop_server()


def another_python_func(param_a, param_b):
    import tsfresh

    # Do data transformation using the packages which
    # are not included in PyODPS node but included in
    # the function image.
```

#### Function Standards

The parameters of the python function for `TO RUN` contain two parts:

1. Context from the SQLFlow statement, it contains the following required fields.

- db_type: mysql/hive/maxcompute, from sqlflowserver configuration.
- input_table: the table name from the clause `SELECT * FROM input_table`.
- output_table: the table name from the clause `INTO output_table`.
- image_name: the docker image name from the clause `TO RUN image_name/func_name`.

2. Attributes from `WITH` clause.

Please check the following example. The first four parameters are from the context and param_1 ~ param_n are from `WITH` Attributes.

```Python
def a_python_func(
    db_type,
    input_table,
    output_table,
    image_name,
    param_1,
    param_2,
    ...,
    param_n
):
    pass
```

### How to invoke TO RUN function

#### Kubernetes

```BASH
docker run a_data_scientist/functions:1.0 python sqlflow.run.submitter.k8s --func_name my_func.data_preprocessor.a_python_func --param_a value_a --param_b value_b --input_table itable --output_table otable --image_name a_data_scientist/functions:1.0 --database hive
```

What `sqlflow.run.submitter.k8s` does:

1. Load the python module `my_func.data_preprocessor` and get the function object `a_python_func`.
2. Invoke the function `a_python_func` with the parameters in the command line above.

#### MaxCompute

```BASH
docker run a_data_scientist/functions:1.0 python sqlflow.run.submitter.alisa --func_name my_func.data_preprocessor.a_python_func --param_a value_a --param_b value_b --input_table in_table --output_table out_table --image_name a_data_scientist/functions:1.0 --database maxcompute
```

What `sqlflow.run.submitter.alisa` does:

1. Load the python module `my_func.data_preprocessor` and get the function object `a_python_func`.
2. Get source code of `a_python_func` using inspect.getsource(a_python_func).
3. Generate a line of code to invoke `a_python_func` with the parameters from the command line above and append it to the code from step 2.
4. Submit a PyODPS task using alisa, the generated code is a parameter of the web request to alisa.
5. Wait for the PyODPS task done.
