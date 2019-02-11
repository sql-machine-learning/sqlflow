# Piping Responses


## Streaming Responses

As described in the [overall design](doc/design.md), a SQLFlow job could be a standard or an extended SQL statemnt, where an extended SQL statement will be translated into a Python program.  Therefore, each job might generate up to the following data streams:

1. standard output, where each element is a line of text,
1. standard error, where each element is a line of text,
1. data rows, where the first element consists of fields name/types, and each of the rest is a row of data,
1. status, where the element could be *pending*, *failed*, and *succeeded*.

To create good user experience, we need to pipe these responses from SQLFlow jobs to Jupyter Notebook in real-time.


## Stages in the Pipe

The pipe that streams outputs from SQLFlow jobs to the Jupyter Notebook consists of the following stages:

```
Web browser 
 ↑
 | HTTP
 ↓
Jupyter Notebook server
 ↑
 | ZeroMQ streams: Shell, IOPub, stdin, Controls, Heartbeat
 ↓
iPython kernel
 ↑
 | IPython magic command framework
 ↓
SQLFlow magic command for Jupyter
 ↑
 | gRPC
 ↓
SQLFlow server
 ↑
 | Go channels
 ↓
SQLFlow job manager (Go functions)
 ↑
 | IPC with Go's standard library
 ↓ 
SQLFlow jobs
```

In the above figure, from the SQLFlow magic command to the bottom layer are our work.


##  Streaming

We have two alternative ideas: multiple streams and a multiplexing stream.
We decided to use a multiplexing stream because we had a unsuccessful trial with the multiple streams idea: we make the job writes to various Go channels and forward each Go channel to a streaming gRPC call, as the following:

### Multiple streams

The above figure shows that there are multiple streams between the Jupyter Notebook server and Jupyter kernels.  According to the [document](https://jupyter-client.readthedocs.io/en/stable/messaging.html), there are five: Shell, IOPub, stdin, Control, and Heartbeat.  These streams are ZeroMQ streams.  We don't use ZeroMQ, but we can take the idea of having multiple parallel streams in the pipe.


```protobuf
service SQLFlow {
    rpc File(string sql) returns (int id) {}

    rpc ReadStdout(int id) returns (stream string) {}
    rpc ReadStderr(int id) returns (stream string) {}
    rpc ReadData(int id) returns (stream Row) {}
    rpc ReadStatus(int id) returns (stream int) {}
}
```

However, we realized that if the user doesn't call any one of the `SQLFlow.Read...` call, there would be no forwarding from the Go channel to Jupyter, thus the job would block forever at writing.

## A Multiplexing Stream

Another idea is multiplexing all streams into one. For example, we can have only one ZeroMQ stream, where each element is a polymorphic type -- could be a text string or a data row.

```protobuf
service SQLFlow {
    rpc Run(string sql) returns (stream Response) {}
}

// Only one of the following fields should be set.
message Response {
    oneof record {
        repeated string head = 1;             // Column names.
        repeated google.protobuf.Any row = 2; // Cells in a row.
        string log = 3;                       // A line from stderr or stdout.
    }
}
```
