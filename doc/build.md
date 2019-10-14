# Build from Source in a Docker Container

Referring to [this example](https://github.com/wangkuiyi/canonicalize-go-python-grpc-dev-env),
we create a canonical dev environment for Go and Python developers using Docker images.

## Editing on Host

When we use this Docker image for daily development work, the source code relies
on the host computer instead of the container. The source code includes this repo
and all its dependencies, for example, the Go package `google.golang.org/grpc`.
Code-on-the-host allows us to run our favorite editors (Emacs, VIM, Eclipse, and more)
on the host.  Please free to rely on editors add-ons to analyze the source code
for auto-completion.

## Building in Container

We build a Docker image that contains development tools below.

1. Python Interpreter
1. Go compiler
1. Protobuf compiler
1. Protobuf to Go compiler extension
1. Protobuf to Python compiler extension

Because this repo contains Go code, please make sure that you have the directory structure required by Go. On my computer, I have GOPATH set to $Home/go, you can have your `$GOPATH` pointing to any directory as you like.

```bash
export GOPATH=$HOME/go
```

Now that `$GOPATH$` is set, we could git clone the source code of our project by running:

```bash
go get sqlflow.org/sqlflow
```

Change the directory to our project root, and we can use `go get` to retrieve
and update Go dependencies. Note `-t` instructs get to also download the packages required to build
the tests for the specified packages. As all Git users would do, we run `git pull` from time to time to sync up with
others' work. If somebody added new dependencies, we might need to run `go -u ./...`
after `git pull` to update dependencies.

```bash
cd $GOPATH/src/sqlflow.org/sqlflow
go get -u -t ./...
```

To build the project, we need protobuf compiler, Go compiler, Python interpreter and gRPC extension to protobuf compiler. To prepare our dev environment with these tools, the easist way is to pull latest image from DockerHub by running command below and give it an alias sqlflow:latest. Alternatively, we provide a Dockerfile where can build image from. Note it will take a while to build from Dockerfile, especially when the network is unpredictable.

```bash
docker pull sqlflow/sqlflow
docker tag sqlflow/sqlflow:latest sqlflow:latest
```

or

```bash
docker build -t sqlflow:latest .
```

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

```
service mysql start
```

run all the tests as

```
go generate ./...
SQLFLOW_TEST_DB=mysql go test -v ./...
```

where `go generate` invokes the `protoc` command to translate `server/sqlflow.proto`
into `server/sqlflow.pb.go` and `go test -v` builds and run unit tests. The environment variable
`SQLFLOW_TEST_DB=mysql` specify MySQL as the backend, you can also check [test_hive.sh](https://github.com/sql-machine-learning/sqlflow/blob/develop/scripts/test_hive.sh) and
[test_maxcompute.sh](https://github.com/sql-machine-learning/sqlflow/blob/develop/scripts/test_maxcompute.sh) to run the unit tests with other backends.

## The Command-line REPL

The REPL is a binary linked with SQLFlow. In the Docker image, the sample data is already loaded in
the MySQL service, you can start MySQL using `service mysql start`. To run it, type the following
command:

```bash
go run cmd/repl/repl.go --datasource="mysql://root:root@tcp(host.docker.internal:3306)/?maxAllowedPacket=0"
```

You should be able to see the following prompt

```
sqlflow>
```

Then, please follow the [REPL tutorial](/doc/run/repl.md).
