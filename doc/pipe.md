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


##  Streams in the Pipe

### Multiple Streams

The above figure shows that there are multiple streams between the Jupyter Notebook server and Jupyter kernels.  According to the [document](https://jupyter-client.readthedocs.io/en/stable/messaging.html), there are five: Shell, IOPub, stdin, Control, and Heartbeat.  These streams are ZeroMQ streams.  We don't use ZeroMQ, but we can take the idea of having multiple parallel streams in the pipe.

### Multiplexing Stream

Another idea is multiplexing all streams as one.  For example, we can have only one ZeroMQ stream, where each element is a polymorphic type -- could be a text string or a data row.  To be precise, multiplexing is not an alternative idea to multi-streams, but an addition -- in cases where we could have only one stream, we might have to multiplex information.  In this document, we don't have that constraint.


## gRPC

Let us start from the gRPC between SQLFlow magic command and SQLFlow server.  gRPC supports [server streaming](https://grpc.io/docs/guides/concepts.html#server-streaming-rpc).  So we could design the SQLFlow gRPC service as

```protobuf
service SQLFlow {
    rpc File(string sql) returns (int id) {}

    rpc ReadStdout(int id) returns (stream string) {}
	rpc ReadStderr(int id) returns (stream string) {}
	rpc ReadData(int id) returns (stream Row) {}
    rpc ReadStatus(int id) returns (stream int) {}
}
```

Please be aware that to make the design doc concise, I slightly broke the syntax of protobuf gRPC definition, without lossing the expressiveness hopefully.

The `File` call launches a job and returns a job ID, given which, the magic command could read various streams via the `Read...` calls.


## Job Management

To maintain the job ID, we need a Go type `jobRegistry` for the SQLFlow server:

```go
type jobRegistry map[uint64]*job
```

where the job definition consists of its interfaces as three streams:

```
type job struct {
   stderr chan string
   stdout chan string
   data chan Row
   status chan int
}
```

The implementation of `SQLFlow.File` should launch a job, creates a `job` variable, and registers it into the registry, so that the implementations of `SQLFlow.Read...` could read from the Go channels and forward to gRPC streams.


## Job with Details

A job is indeed a goroutine.  We have two functions that can run as goroutines:

1. `runStadnardSQL`, which forwards the SQL statement to the SQL engine (e.g., MySQL), writing status into the `job.status` channel,  and iterates the rows enumerator and writes data into the `job.data` channel.

1. `runExtendedSQL`, which translates the SQL statement into Python and runs the Python.  It relies on Go's standard library APIs like https://golang.org/pkg/os/exec/#Cmd.StderrPipe to capture the stderro and stdout of the subprocess and writes the text into `job.stderr` and `job.stdout` channels.

The above functions creates a `job` of channles, launches a goroutine to do the work, and returns the `job` immediately after the launching of the goroutine.

