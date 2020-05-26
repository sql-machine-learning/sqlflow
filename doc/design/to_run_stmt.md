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
TO RUN a_data_scientist/maxcompute_functions:1.0/data_preprocessor.a_python_func
WITH param_a = value_a,
     param_b = value_b
INTO result_table
```

The SQLFlow statement above will call the python function `a_python_func` from the file `/run/data_preprocessor.py` inside the docker image `a_data_scientist/maxcompute_functions:1.0`. The attributes in the `WITH` clause will be passed to the python function as parameters.

## Challenges

- TO RUN can execute functions in Kubernetes and MaxCompute.
- TO RUN can execute functions in both a single process and a distributed cluster.
- TO RUN can execute functions built upon various data computing frameworks such as [Dask](https://github.com/dask/dask), [Mars](https://github.com/mars-project/mars), [Ray](https://github.com/ray-project/ray) and so on.

## Design

### Fix the challenges

The semantics of `TO RUN a_data_scientist/maxcompute_functions:1.0/data_preprocessor.a_python_func` means that `a_python_func` will be executed in a docker container for Kubernetes or a PyODPS node for MaxCompute. The implementation of `a_python_func` is fully customized by users. Because the execution environments for `a_python_func` are different between Kubernetes and MaxCompute, there are some differences considering these three challenges above.

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
  Since PyODPS node cannot guarantee it contains all the packages we need, `a_python_func` only create a mars cluster containing one worker and tell this worker to execute another function `another_python_func` to do the data transformation via [Mars remote API](https://github.com/mars-project/mars/issues/1227). The cluster uses our function image `a_data_scientist/maxcompute_functions:1.0`, `another_python_func` and its dependencies are already installed while building this image.
  
- Distributed cluster execution  
  The PyODPS node only installed Mars package, but not Dask and Ray. So we can create a cluster, build the computing graph and submit the distributed execution only using Mars API inside `a_python_func`.

### How to build TO RUN function

#### Structure

Kubernete

```TXT
-- data_process
---- tsfresh_extractor.py
-- Dockerfile
```

MaxCompute

```TXT
-- data_process
---- tsfresh_extractor.py
-- Dockerfile
```

#### Function Standards

### How to invoke TO RUN function

The paramters passed into the python module contains two parts:

1. Context.

- input_table
- output_table
- image_name

2. Parameters from `WITH` clause.

Kubernetes

```BASH
docker run a_data_scientist/functions:0.1 python /run/main.py --func_name data_proc --param_a value_a --param_b value_b
```

MaxCompute

1. Generate `main.py` from `main.template`.
2. Submit a PyODPS task to MaxCompute via goalisa. The content of generated `main.py` is one parameter of the alisa request.

### Integration with Mars and TSFresh
