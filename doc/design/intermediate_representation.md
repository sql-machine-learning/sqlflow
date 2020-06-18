# Intermediate Representation

## Overview

As SQLFlow is supporting more and more machine learning toolkits, the corresponding code generation logics are better being organized as separate packages. An intermediate representation(IR) of the SQL jobs becomes necessary to connect these separate packages with the core `sql` package.

The core `sql` package should include the following functionalities:
1. The entry point of running extended SQL statements.
1. The [parsing](https://github.com/sql-machine-learning/sqlflow/blob/develop/doc/design/sql_parser.md) of extended SQL statements.
1. The verification of extended SQL statements, including verifying the syntax, the existence of the selected fields.
1. The [feature derivation](https://github.com/sql-machine-learning/sqlflow/blob/develop/doc/design/feature_derivation.md), including name, type, shape, and preprocessing method of the select fields.
1. The [training data and validation data split](https://github.com/sql-machine-learning/sqlflow/blob/develop/doc/design/training_and_validation.md).

With these functionalities, the `sql` package çan translate user typed extended SQL statements to an IR as an exposed Go struct. The codegen package takes the IR and returns a generated Python program for the `sql` package to execute.

## Code Structure

We propose the following code structures.

```
sql/
  ...
  codegen/
    feature_column.go
    intermediate_representation.go
    tensorflow/
      ...
    xgboost/
      ...
```

The IR and feature column definition will resides in `codegen`. Each code generator package forms a subdirectory in `codegen` like `codegen/tensorflow/`.

## Intermediate Representation

Please refer to [codegen/intermediate_representation.go](/pkg/codegen/intermediate_representation.go) and [codegen/feature_column.go](/pkg/codegen/feature_column.go) for implementation details.
