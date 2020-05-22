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

SQLFlow compiles the above SQL program into an execution plan and runs it.  As the TRAIN statement above, SQLFlow uses `TO TRAIN CLAUSE`  to train a specific model called  `DNNClassifier`, using `WITH CLAUSE` to configure the training arguments. 

Sometimes users may make some configuration mistake on `WITH CLAUSE`, then the job would fault during execution and return some uncertain error message.

This documentation issued a way that adding extra contract in docstring to contract the arguments, and this can achieve three advantage at least:

1. Early testing, we can do early testing before running the job; users can wait less time and cluster save resources.
2. More accurate diagnostic message.
3. Using docstring is Keras or Tensorflow native; users don't need to modify the model code.

## Design

The docstring of a function or a class can include argument documentation and contracts.

The argument documentation starts with argument name and description followed by a colon `:` and the contrast on this line starts with `#`. The contract should be Python code and return `True` or `False`.

An example:

```python 
class MyDNNClassifier(keras.Model)
        def __init__(self, n_classes=32, hidden_units=[32, 64])
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

If a user enter some invalide arguments:

``` sql
SELECT ... TO TRAIN sqlflow_models.MyDNNClassifier
WITH
    model.n_classes=1,
    model.hidden_units=64
LABEL class
INTO my_dnn_model;
```

We expected the SQLFlow GUI show the following error message:

``` text
SQLFLow received attribute error:
> model.n_classes received unexpected value: 1, attribute usage:
Number of label classes. Defaults to 2, namely binary classification. Must be > 1.

> model.hidden_units received unexpected value: 64, attribute usage:
Iterable of number hidden units per layer. All layers are fully connected. Ex. `[64, 32]` means first layer has 64 nodes and second one has 32.
```

We can extract the argument contract and documentation from the docstring, and check it on the compile phase.

``` python
def attribute_check(estimator, **args):
      # extract argument name, documentation and contracst from doc string  
      contrcast = extract_doc_string(estimator)
    # SQLFlowDIagnosticError message can be pipe to SQLFlow GUI via SQLFlow gRPC server
    diag_err = SQLFLowDiagnosError()
    for name, value in args:
      if ! contracst.check(name, value):
            # component received value and argument documentation
              diag_err.append_message(contracst.diag_message(name, value))
    if ! diag_err.empty():
          raise diag_err
```
