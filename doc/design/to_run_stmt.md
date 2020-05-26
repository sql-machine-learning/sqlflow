# To Run Statement

## The Problem

SQLFlow extends the SQL syntax and can describe the end-to-end machine learning pipeline. Data transformation is an important part in the entire pipeline. Currently we have the following two options for data transformation:

- SQL.
- COLUMN clause for per data instance transform and the logic is saved into the model.

But that's not enough. It's a common case that the transformation is not per data instance and we cannot use SQL or need write very complex SQL to express the transfomration logic. For example: `TSFresh.extract_features`.

We can express the transform logic using Python and encapsulate it into a reusable module. And then we propose to add the `TO RUN` clause in SQLFlow to invoke it.

## Syntax Extension

```SQL
SELECT * FROM source_table
TO RUN a_data_scientist/functions:0.1/data_proc
WITH param_a = value_a,
     param_b = value_b
INTO result_table
```

The SQLFlow statement above will execute the python module in the folder `/data_proc` from the docker image `a_data_scientist/functions:0.1`.

## Challenges

- TO RUN can execute functions in both a single process and a distributed cluster.
- TO RUN can execute functions in Kubernetes and MaxCompute.
- TO RUN can execute functions built upon various data computing frameworks such as [Dask](https://github.com/dask/dask), [Mars](https://github.com/mars-project/mars), [Ray](https://github.com/ray-project/ray) and so on.

## Design

### What is TO RUN function

Kubernete

```TXT
-- data_proc
---- main.py
---- util_lib.py
-- Dockerfile
```

MaxCompute

```TXT
-- data_proc
---- main.template
---- util_lib.py
-- Dockerfile
```

### How to invoke module TO RUN

The paramters passed into the python module contains two parts:

1. Context.

- table_name
- image_name

2. Parameters from `WITH` clause.

Kubernetes

```BASH
docker run a_data_scientist/functions:0.1 python /data_proc/main.py --param_a value_a --param_b value_b
```

MaxCompute

1. Generate `main.py` from `main.template`.
2. Submit a PyODPS task to MaxCompute via goalisa. The content of generated `main.py` is one parameter of the alisa request.

### Function Standards

### How to contribute TO RUN module

### TSFresh Integration
