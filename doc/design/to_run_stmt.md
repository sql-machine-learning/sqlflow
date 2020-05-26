# To Run Statement

## The Problem

SQLFlow extends the SQL syntax and can describe an end-to-end machine learning pipeline. Data transformation is an important part in the entire pipeline. Currently we have the following two options for data transformation:

- SQL.
- COLUMN clause for per data instance transform. The transform logic is saved into the model.

But that's not enough. It's a common case that the transformation is not per data instance which COLUMN clause is not suitable for. And we may need write very complex SQL or even cannot use SQL to express the transfomration logic. Let's take `extract_features` from [tsfresh package](https://tsfresh.readthedocs.io/en/latest/api/tsfresh.feature_extraction.html#module-tsfresh.feature_extraction.extraction) for example. It will preprocess the time series data, calculate statistical values using various analysis functions and parameters, and then generate tens or hunderds of features.

We can express the transform logic using Python, leverage many mature python packages and encapsulate it into a reusable function. And then we propose to add the `TO RUN` clause in SQLFlow to call it.

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

- TO RUN can execute functions in both a single process and a distributed cluster.
- TO RUN can execute functions in Kubernetes and MaxCompute.
- TO RUN can execute functions built upon various data computing frameworks such as [Dask](https://github.com/dask/dask), [Mars](https://github.com/mars-project/mars), [Ray](https://github.com/ray-project/ray) and so on.

## Design

### What is TO RUN function

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

### How to invoke module TO RUN

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

### Function Standards

### How to contribute TO RUN module

### TSFresh Integration
