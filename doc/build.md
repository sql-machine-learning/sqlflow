# Build from Source in a Docker Container

We create a canonical dev environment for Go and Python developers using Docker images.

## Prerequisite

1. Go >= 1.13. [download here](https://golang.org/dl/)
1. Docker CE >= 18.x. [download here](https://docs.docker.com/docker-for-mac/install/)

## Building in Container

We build a Docker image that contains development tools below.

1. Python Interpreter
1. Go compiler
1. Protobuf compiler
1. Protobuf to Go compiler extension
1. MySQL database

Because this repo contains Go code, please make sure that you have the directory structure required by Go. On my computer, I have `GOPATH` set to `$Home/go`, you can have your `$GOPATH` pointing to any directory as you like.

```bash
export GOPATH=$HOME/go
```

Now that `$GOPATH$` is set, we could git clone the source code of our project by running:

```bash
go get sqlflow.org/sqlflow
```

Change the directory to our project root by running

```bash
cd $GOPATH/src/sqlflow.org/sqlflow
```

Build the Docker image either from project `Dockerfile`.

```bash
docker build -t sqlflow:latest .
```

or SQLFlow's [official registry](https://hub.docker.com/r/sqlflow/sqlflow/tags) on DockerHub.

```bash
docker pull sqlflow/sqlflow
docker tag sqlflow/sqlflow:latest sqlflow:latest
```

Note it will take a while to build from Dockerfile, especially when the network is unpredictable.

## Development

### Build and Test

We build and test the project inside the docker container. To run the container, we need to map the `$GOPATH` directory on the host into the
`/go` directory in the container, because the Dockerfile configures `/go` as
the `$GOPATH` in the container:

```bash
docker run --rm -it -v $GOPATH:/go \
    -w /go/src/sqlflow.org/sqlflow \
    sqlflow:latest bash
```

Inside the Docker container, start a MySQL server in the background

```bash
service mysql start
```

run all the tests as

```bash
go generate ./...
SQLFLOW_TEST_DB=mysql go test -v ./...
```

where `go generate` invokes the `protoc` command to translate `server/sqlflow.proto` into `server/sqlflow.pb.go` and `go test -v` builds and run unit tests. The environment variable `SQLFLOW_TEST_DB=mysql` specify MySQL as the backend, you can also check [test_hive.sh](https://github.com/sql-machine-learning/sqlflow/blob/develop/scripts/test_hive.sh) and [test_maxcompute.sh](https://github.com/sql-machine-learning/sqlflow/blob/develop/scripts/test_maxcompute.sh) to run the unit tests with other backends.

## Editing on Host

When we use this Docker image for daily development work, the source code relies on the host computer instead of the container. The source code includes this repo and all its dependencies, for example, the Go package `google.golang.org/grpc`. Code-on-the-host allows us to run our favorite editors (Emacs, Vim, Eclipse, and more) on the host.  Please free to rely on editors add-ons to analyze the source code for auto-completion.

## The Command-line REPL

The REPL is a binary linked with SQLFlow. In the Docker image, the sample data is already loaded in the MySQL service, you can start MySQL using `service mysql start`. To run it, type the following
command:

```bash
go run cmd/repl/repl.go --datasource="mysql://root:root@tcp(localhost:3306)/?maxAllowedPacket=0"
```

You should be able to see the prompt of `sqlflow>`.  Now, please follow the [REPL tutorial](run/repl.md).
