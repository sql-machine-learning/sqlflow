# Diagnostic Attribute Error

## Motivation

SQLFlow extended SQL syntax to describe an end-to-end machine learning job, for a typical SQL program:

``` python
SELECT * FROM iris.train
TO TRAIN DNNClassifier
WITH
  model.n_classes = 2,
  model.hidden_units = [128, 64]
LABEL class
INTO my_model;

SELECT * FROM iris.test
TO PREDICT iris.pred.class
USING my_model;
```

SQLFlow compiles each statement in the program into an execution plan and executes them.
As the TRAIN statement above, SQLFlow uses `TO TRAIN` clause to train a specific model called `DNNClassifier`,
using `WITH` clause to configure the training arguments.

Sometimes users may make some configuration mistake on `WITH` clause,
then the job would fail during execution and return some uncertain error message.

The model parameter documentation describes parameters and acceptable values of human reading.
We want to enhance it for the reading by the SQLFlow compiler so to warn about wrongly set parameters, and
this can active three advantages at least:

1. Early testing, we can do early testing before running the job; users can wait less time and cluster save resources.
2. More accurate diagnostic message.
3. Model developers do not have to involve dependencies other than Keras or TensorFlow.

## Design

We want to document the compiler-readable description of model parameters in the docstring of
Python function or class that define a model.

A docstring contains multiple lines:

- A line starting with `#` is the check rule in Python code.
- A line starting with argument name and document followed by a colon `:`.

An example:

```python

class MyDNNClassifier(keras.Model)
    def __init__(self, n_classes=32, hidden_units=[32, 64]):
    """
    Args:

    # isintance(n_classes, int) && n_classes > 1
    n_classes: Number of label classes. Defaults to 2, namely binary
    classification. Must be > 1.

    # isintance(hidden_units, list) && all(isinstance(item, int) for item in hidden_units)
    hidden_units: Iterable of number hidden units per layer. All layers are
    fully connected. Ex. `[64, 32]` means first layer has 64 nodes and
    second one has 32.
    """
```

If a user set some invalid parameters as the following SQL statement:

``` sql
SELECT ... TO TRAIN sqlflow_models.MyDNNClassifier
WITH
    model.n_classes=1,
    model.hidden_units=64
LABEL class
INTO my_dnn_model;
```

We expected the SQLFlow GUI show the error message as:

``` text
SQLFLow received attribute error:
> model.n_classes received unexpected value: 1, attribute usage:
Number of label classes. Defaults to 2, namely binary classification. Must be > 1.

> model.hidden_units received unexpected value: 64, attribute usage:
Iterable of number hidden units per layer. All layers are fully connected. Ex. `[64, 32]` means first layer has 64 nodes and second one has 32.
```

For the implementation, it's easy to extract the check rule and argument documentation from the docstring, and check it on the compile phase.

``` python
def attribute_check(estimator, **args):
    # extract argument name, documentation and contract from doc string  
    contract = extract_symbol(estimator)
    # SQLFlowDiagnosticError message can be pipe to SQLFlow GUI via SQLFlow gRPC server
    diag_err = SQLFLowDiagnosError()
    for name, value in args:
      if !contract.check(name, value):
            # component received value and argument documentation
              diag_err.append_message(contract.diag_message(name, value))
    if !diag_err.empty():
          raise diag_err
```

## Future

This documentation using native Python code to express the check rule,
[another PR](https://github.com/sql-machine-learning/sqlflow/pull/2245) designed a new Python library to make the code shorter and simpler, will make more discussion in the future.
