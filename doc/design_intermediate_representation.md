# _Design:_ Intermediate Representation

## Overview

As SQLFlow is supporting more and more machine learning toolkits, their corresponding code generation logics are better being orgnized as separate packages. An intermediate representation(IR) of the SQL jobs becomes necessary to connect these separate packages with the core `sql` package.

The core `sql` package should include the following functionalities:
1. The entry point of running extended SQL statements.
1. The [parsing](https://github.com/sql-machine-learning/sqlflow/blob/develop/doc/sql_parser.md) of extended SQL statements.
1. The verification of extended SQL statements, including verifying the syntax, the existence of the selected fields.
1. The [feature derivation](https://github.com/sql-machine-learning/sqlflow/blob/develop/doc/feature_derivation.md), including name, type, shape, and preprocessing method of the select fields.
1. The [training data and validation data split](https://github.com/sql-machine-learning/sqlflow/blob/develop/doc/training_and_validation.md).

With these functionalities, the `sql` package çan translate user typed extended SQL statements to an IR as an exposed Go struct. The codegen package takes the IR and returns a generated Python program for the `sql` package to execute.

## Code Structure

We propose the following code structures.

```
sql/
  ...
  codegen/
    tensorflow/
      train.go
      predict.go
      analyze.go
    xgboost/
      ...
```

The `tensorflow` package will expose function `func Train(ir sql.TrainIR) string, error`, which takes the `sql`'s `TrainIR` and returns a generated Python program.

## Intermediate Representation

We propose the following struct as the IR for code generation.

```go
package sql

import (
	"github.com/sql-machine-learning/sqlflow/sql/columns"
)

type FieldType int

const (
	Int FieldType = iota
	Float
	String
)

// FieldMeta contains the meta information for decoding and feature columns
type FieldMeta struct {
	DType         FieldType               // e.g. "float", "int32"
	Delimiter     string                  // e.g. ","
	Shape         []int                   // e.g. [1], [1 2 3]
	IsSparse      bool                    // e.g. false
	FeatureColumn []columns.FeatureColumn // e.g. [EmbeddingColumn, CategoryIDColumn]
}

// TrainIR is the intermediate representation for code generation of a training job
type TrainIR struct {
	DataSource       string                          // e.g. "hive://root:root@localhost:10000/churn"
	Select           string                          // e.g. "select * from iris.train"
	ValidationSelect string                          // e.g. "select * from iris.val;"
	Estimator        string                          // e.g. "DNNClassifier"
	Attribute        map[string]interface{}          // e.g. {"train.epoch": 1000, "model.hidden_units": [10 10]}
	Feature          map[string]map[string]FieldMeta // e.g. {"feature_columns": {"sepal_length": {"float", "", [1], false}, ...}}
	Label            map[string]FieldMeta            // e.g. {"class": {"int32", "", [1], false}}
}

// PredictIR is the intermediate representation for code generation of a prediction job
type PredictIR struct {
	DataSource  string                          // e.g. "hive://root:root@localhost:10000/churn"
	Select      string                          // e.g. "select * from iris.test"
	Estimator   string                          // e.g. "DNNClassifier"
	Attribute   map[string]interface{}          // e.g. {"predict.batch_size": 32}
	Feature     map[string]map[string]FieldMeta // e.g. {"feature_columns": {"sepal_length": {"float", "", [1], false}, ...}}
	Label       map[string]FieldMeta            // e.g. {"class": {"int32", "", [1], false}}
	ReusltTable string                          // e.g. "iris.predict"
}

// AnalyzeIR is the intermediate representation for code generation of a analysis job
type AnalyzeIR struct {
	DataSource string                          // e.g. "hive://root:root@localhost:10000/churn"
	Select     string                          // e.g. "select * from iris.train"
	Estimator  string                          // e.g. "DNNClassifier"
	Attribute  map[string]interface{}          // e.g. {"analyze.plot_type": "bar"}
	Feature    map[string]map[string]FieldMeta // e.g. {"feature_columns": {"sepal_length": {"float", "", [1], false}, ...}}
	Label      map[string]FieldMeta            // e.g. {"class": {"int32", "", [1], false}}
}
```

Please be aware that all the IR excludes the information of working directory. This information belongs to the `executor` in `sql` package.
- For training job
  - If `executor` runs the generated program in a temporary directory, it should serialize the directory to a table for later use.
  - If `executor` runs the generated program in a local directory, it should make sure the prediction and analyze job sees the same directory.
- For prediction and analyze job, the `executor` should recover everything produced by the training job.

Please be aware that `TrainIR` excludes the saving table name. This information belongs to the `executor` in `sql` package.
- For a local training job, the result of the generated program contains the trained model. And `executor` is re
- For a distributed training job, the generated program should garantee that the local directory contains enough information, such as OSS bucket name. So that later on the prediction job find it.
