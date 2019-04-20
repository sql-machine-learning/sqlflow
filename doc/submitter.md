# Submitter

A submitter is a pluggable module in SQLFlow that is used to submit an ML job to a third party computation service.

## Workflow

When a user types in an extended SQL statement, SQLFlow first parses and semantically verifies the statement. Then SQLFlow either runs the ML job locally or submits the ML job to a third party computation service. 

![](doc/figures/sqlflow-arch2.png)

In the latter case, SQLFlow produces a job description (`TrainDescription` or `PredictDescription`) and hands it over to the submitter. For a training SQL, SQLFlow produces `TrainDescription`; for prediction SQL, SQLFlow produces `PredDescription`. The concrete definition of the description looks like the following

```go
type ColumnType struct {
    Name             string // e.g. sepal_length
    DatabaseTypeName string // e.g. FLOAT
}

// SELECT *
// FROM iris.train
// TRAIN DNNClassifier
// WITH
//   n_classes = 3,
//   hidden_units = [10, 20]
// COLUMN sepal_length, sepal_width, petal_length, petal_width
// LABEL class
// INTO sqlflow_models.my_dnn_model;
type TrainDescription struct {
    StandardSelect string       // e.g. SELECT * FROM iris.train
    Estimator      string       // e.g. DNNClassifier
    Attrs          map[string]string // e.g. "n_classes": "3", "hidden_units": "[10, 20]"
    X              []ColumnType // e.g. "sepal_length": "FLOAT", ...
    Y              ColumnType   // e.g. "class": "INT"
    ModelName      string       // e.g. my_dnn_model
}

// SELECT *
// FROM iris.test
// PREDICT iris.predict.class
// USING sqlflow_models.my_dnn_model;
type PredDescription struct {
    StandardSelect string // e.g. SELECT * FROM iris.test
    TableName      string // e.g. iris.predict
    ModelName      string // e.g. my_dnn_model
}
```

## Submitter Interface

The submitter interface should provide two functions `Train` and `Predict`. The detailed definition can be the following

```go
type Submitter interface {
    // Train executes a ML training job and streams job's response through writer.
    // A typical Train function should include
    // - Loading the training data
    // - Initializing the model
    // - model.train
    // - Saving the trained model to a persistent storage
    Train(desc TrainDescription, writer PipeWriter) error
    // Predict executes a ML predicting job and streams job's response through writer
    // A typical Predict function should include
    // - Loading the model from a persistent storage
    // - Loading the prediction data
    // - model.predict
    // - Writing the prediction result to a table
    Predict(desc PredictDescription, writer PipeWriter) error
}
```

## Register a submitter

A new submitter can be added as

```go
import (
    ".../my_submitter"
    ".../sqlflow/sql"
)

func main() {
    // ...
    sql.Register(my_submitter.NewSubmitter())
    // ...
    for {
    	sql := recv()
    	sql.Run(sql)
    }
}
```

where `sql.Register` will put `my_submitter` instance to package level registry. During `sql.Run`, it will check whether there is a submitter registered. If there is, `sql.Run` will run either `submitter.Train` or `submitter.Predict`.
