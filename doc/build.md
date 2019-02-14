# Canonical Development Environment

Referring to [this example](https://github.com/wangkuiyi/canonicalize-go-python-grpc-dev-env),
we create a canonical development environment for Go and Python programmers using Docker.

### Editing on Host

When we use this Docker image for daily development work, the source code relies
on the host computer instead of the container. The source code includes this repo
and all its dependencies, for example, the Go package `google.golang.org/grpc`.
Code-on-the-host allows us to run our favorite editors (Emacs, VIM, Eclipse, and more)
on the host.  Please free to rely on editors add-ons to analyze the source code
for auto-completion.

### Building in Container

We build a Docker image that contains development tools:

1. The Python interpreter
1. The Go compiler
1. The protobuf compiler
1. The protobuf to Go compiler extension
1. The protobuf to Python compiler extension

Because this repo contains Go code, please make sure that you have the directory
structure required by Go. On my laptop computer, I have

```bash
export GOPATH=$HOME/go
```

You could have your `$GOPATH` pointing to any directory you like.

Given `$GOPATH$` set, we could git clone the source code of our project by running:

```bash
go get -insecure gitlab.alipay-inc.com/Arc/sqlflow
```

To build this project, we need the protobuf compiler, Go compiler, Python interpreter,
gRPC extension to the protobuf compiler. To ease the installation and configuration
of these tools, we provided a Dockerfile to install them into a Docker image.
To build the Docker image:

```bash
cd $GOPATH/src/gitlab.alipay-inc.com/Arc/sqlflow
docker build -t grpc -f Dockerfile.dev .
```

## How to Build and Test

We build and test the project inside the docker container.

To run the container, we need to map the `$GOPATH` directory on the host into the
`/go` directory in the container, because the Dockerfile configures `/go` as
the `$GOPATH` in the container:

```bash
docker run --rm -it -v $GOPATH:/go \
    -w /go/src/gitlab.alipay-inc.com/Arc/sqlflow \
    --net=host
    grpc bash
```

### server

To test the `./server`:

```bash
cd server
go get -u ./...
go generate
go test -v
```

where the `go get -u ./...` retrieves and updates Go dependencies of our server,
`go generate` invokes the `protoc` command to translate `server/sqlflow.proto` into
`server/sqlflow.pb.go`, `go install` builds the server into `$GOPATH/bin/server`,
and `go test -v` builds and run unit tests, which runs the gRPC server in a goroutine
and the client in another goroutine.

### sqlflow

To test the `./sql`:
 
On a seperate terminal, follow the [guide](example/datasests/README.md) to start a mysql server.

In the develop docker container

```bash
cd sql
go get -u ./...
go test -v
```

### sqlfs

To test the `./sqlfs`:
 
On a seperate terminal, follow the [guide](example/datasests/README.md) to start a mysql server.

In the develop docker container

```bash
cd sqlfs
go get -u ./...
go test -v
```
