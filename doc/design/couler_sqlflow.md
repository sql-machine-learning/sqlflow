# Couler on SQLFlow

## Motivations

The purpose of Couler is to provide a joyful experience of writing workflows runnable on professional engines like Argo. SQLFlow translates a SQL program into a Python machine-learning program in Python.
This architle introduces how does SQLFlow translate a SQLFlow program into a Couler workflow program.

## SQLFlow and Couler

The Couler core package implemented many functions like:
1. `couler.run_container(docker_image, cmd, args)` starts a Docker contanier.
1. `couler.run_python(python_func_name, docker_image="python:3.6")` runs a Python function in the given Docker image.

For SQLFlow, we would like to implement multiple Python function to Train/Predict TensorFlow/XGBoost/... model:

``` python
couler.python_run(python_func_name="xgboost.train", ir=IR)
```

From the above example:

- `xgboost.train` is a Python function which implement XGBoost training code.
- `IR` is SQLFlow intermediate representation with JSON format.

## Couler Code Generator

For the current implementation, SQLFlow has multiple code generators, e.g. XGBoost, Tensorflow and EDL. With Couler,
SQLFlow only needs to implement one `couler_codegen.go`, which generats a Couler program. The current XGBoost or Tensorflow code generator would be implemented as a Python function which accepts the IR object. For the following SQL program example:

``` sql
SELECT * FROM a ...;
SELECT * FROM b ...;
SELECT * FROM ... TO TRAIN xgboost.booster ...;
SELECT * FROM ... TO PREDICT ...;
```

`couler_codegen.go` can translates the above SQL program into a Couler program:

``` python
TRAIN_IR = json.loads({{SQLFLOW_TRAIN_IR}})
PREDICT_IR = json.loads({{SQLFLOW_PREDICT_IR}})

couler.odps.run('''SELECT * FROM a...''')
couler.odps.run('''SELECT * FROM b''')
couler.run_python(python_func=xgboost.train, args=(TRAIN_IR,), docker_image="sqlflow/sqlflow")
couler.run_python(python_func=xgboost.predict, args=(PREDICT_IR,), docker_image="sqlflow/sqlflow")
```

## SQLFLow Python Function
