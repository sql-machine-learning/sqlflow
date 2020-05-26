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

### How to invoke TO RUN function

Kubernetes

MaxCompute

- Upload the script into Dataworks as a resource.
- Submit a PyODPS task via Alisa.

### Function Standards

### How to contribute TO RUN function

### TSFresh Integration
