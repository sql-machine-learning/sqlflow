# SQLFlow: Code Walkthrough

## User Experience

SQLFlow allows users to write SQL programs with extended syntax in Jupyter Notebook or a command-line tool.

The following SQL statements train a TensorFlow model named `DNNClassifier`, which is a Python class derived from [`tf.estimator.Estimator`](https://www.tensorflow.org/api_docs/python/tf/estimator/Estimator):

```sql

SELECT * FROM a_table TRAIN DNNClassifier WITH learning_rate=0.01 INTO sqlflow_models.my_model;
```

And the following statement uses the trained model for prediction.

```sql
SELECT * FROM b_table PREDICT b_table.predicted_label USING sqlflow_models.my_model;
```

Please be aware that the part in the above statements before the extended keyword TRAIN and PREDICT is a standard SQL statement. This feature simplifies the implementation of the SQLFlow system.

## System Implementation

If a SQL statement is of the standard syntax, SQLFlow throws it to the SQL engine and redirects the output to the user; otherwise, SQLFlow translates the statement of extended syntax into a Python program.  Currently, it generates a program that throws the standard-syntax part of SELECT to MySQL, reads the results in the train-loop of a TensorFlow program.  We will talk about how to extend SQLFlow to connect more SQL engines like Oracle, Hive, and SparkSQL, and generates more types of machine learning programs that calls distributed TensorFlow, PyTorch, and xgboost later. Before that, let us explain the system components.

### SQLFlow as a gRPC Server

SQLFlow is a gRPC server, which can connect with multiple clients.  A typical client is [pysqlflow](https://github.com/sql-machine-learning/pysqlflow), the SQLFlow plugin for Jupyter Notebook server.  Another once is a text-based client [/cmd/sqlflowserver/main.go](/cmd/sqlflowserver/main.go).

```
Jupyter Notebook          ---(SQL statements)-->       SQLFlow gRPC server
(SQLFlow magic command)   <--(a stream of messages)--
```

The protobuf definition of the gRPC service is at [/server/proto/sqlflow.proto](/server/proto/sqlflow.proto).  The return of the method `SQLFlow.Run` is a stream of `Reponse`s, where each represents either a table header, a row, or a log message.  The header and rows are usually from a standard SQL statement, for example, SELECT or DESCRIBE, and the log messages are usually from the running of a generated Python program.

### SQLFlow in the gRPC Server

Once the SQLFlow server receives a batch of SQL statements via a gRPC call, it runs the following steps for each statement:

1. the [parser](/sql/sql.y) to generate parsing result,
2. the [verifier](/sql/verifier.go) to verify the semantics given the parsing result,
3. the [code generator](/sql/codegen.go) to generate a Python program, or the *submitter*, from the parsing result,
4. the [executor](/sql/executor.go) that runs the submitter locally.

Step 3. and 4. are only for a SQL statement of extended syntax; otherwise, SQLFlow server proxies the standard-syntax statement to the SQL engine.

The executor calls Go's [standard package](https://godoc.org/os/exec) that captures the stdout and stderr from the submitter process and passing the result back to the gRPC client.  Therefore, it is the responsibility of the submitter to print log messages to its stderr and stdout.

### Minimal Viable Product

In the minimal viable product (MVP) of SQLFlow, the code generator generates a Python program consists of two parts:

1. throw the standard SELECT part in the extended-syntax statement to MySQL via ODBC, and
1. a loop that reads outputs from the run of the SELECT statement and trains the model (or, using a trained model to predict).

The training part calls TensorFlow to update the parameters of the model specified in the TRAIN clause.

### Extensibility

By writing more code generators, we could extend SQLFlow to support more SQL engines, e.g., Hive and Oracle, and use machine learning toolkits, e.g., PyTorch and xgboost, in addition to TensorFlow, on various computing platforms.  You are welcome to add more code generators such as

- `codegen_distributed_tf.go` to generate a submitter program similar to the MVP but runs a distributed TensorFlow training job.
- `codegen_kubernetes_tf.go` to launch a distributed TensorFlow job on a Kubernetes cluster, other than running locally, in the same container as where SQLFlow gRPC server resides.
- `codegen_gcloud_pytorch.go` to launch a submitter that calls PyTorch instead of TensorFlow for training on the Google Cloud.

### Job Management

The current design of the gRPC interface assumes that the connection between the client, e.g., the Jupyter Notebook, and the SQLFlow server keeps alive during the running of the training program.  This assumption is reasonable because even if the user closes her/his Web browser and disconnect to the Jupyter Notebook server, the connection between Jupyter to SQLFlow server might keep alive.  However, this might not be robust enough if the Jupyter Notebook server runs on a user's laptop and gets killed.  In such a case, the gRPC server cannot stream the messages back to the client and would cause the failure of the submitter.

A solution is to change the gRPC interface of SQLFlow server to have a method that files a job and returns immediately, and another method to get a batch of recent messages given a job ID.  We will make a design for that soon.
