# To Run Statement

## The Problem

SQLFlow extends the SQL syntax and can describe the end-to-end machine learning pipeline. Data transformation is an important part in the entire pipeline. Currently we have the following two options for data transformation:

- SQL.
- COLUMN clause for per data instance transform and the logic is saved into the model.

But that's not enough. It's a common case that the transformation is not per data instance and we cannot use SQL or need write very complex SQL to express the transfomration logic. For example: `TSFresh.extract_features`.

We can express the transform logic using Python and encapsulate it into a reusable component. And then we propose to add the `TO RUN` clause in SQLFlow to invoke it.

## The Syntax Extension

```SQL
SELECT * FROM source_table
TO RUN a_python_func
WITH param_a = value_a,
     param_b = value_b
INTO result_table
```

```SQL
SELECT * FROM source_table
TO RUN a_data_scientist/functions:v0.1/a_python_func
WITH param_a = value_a,
     param_b = value_b
INTO result_table
```

## The Challenge

- TO RUN function can be executed using local mode or distributed mode.
- TO RUN function can be exeucted in Kubernetes and MaxCompute.
- TO RUN function can use different frameworks such as Dask/Mars/Ray.

## Design

### What is TO RUN function

### How to invoke TO RUN function

### How to contribute TO RUN function

### TSFresh Integration
