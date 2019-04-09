# Canonical Development Environment

Referring to [this example](https://github.com/sql-machine-learning/canonicalize-go-python-grpc-dev-env),
we create a canonical development environment for Go and Python programmers using Docker.

## Editing on Host

When we use this Docker image for daily development work, the source code relies
on the host computer instead of the container. The source code includes this repo
and all its dependencies, for example, the Go package `google.golang.org/grpc`.
Code-on-the-host allows us to run our favorite editors (Emacs, VIM, Eclipse, and more)
on the host.  Please free to rely on editors add-ons to analyze the source code
for auto-completion.

## Building in Container

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
go get github.com/sql-machine-learning/sqlflow
```

Change the directory to our project root, and we can use `go get` to retrieve
and update Go dependencies.

```bash
cd $GOPATH/src/github.com/sql-machine-learning/sqlflow
go get -u -t ./...
```

Note `-t` instructs get to also download the packages required to build
the tests for the specified packages.

As all Git users would do, we run `git pull` from time to time to sync up with
others' work. If somebody added new dependencies, we might need to run `go -u ./...`
after `git pull` to update dependencies.

To build this project, we need the protobuf compiler, Go compiler, Python interpreter,
gRPC extension to the protobuf compiler. To ease the installation and configuration
of these tools, we provided a Dockerfile to install them into a Docker image.
To build the Docker image:

```bash
docker build -t sqlflow:dev -f Dockerfile.dev .
```

## Development

### Build and Test

We build and test the project inside the docker container.

To run the container, we need to map the `$GOPATH` directory on the host into the
`/go` directory in the container, because the Dockerfile configures `/go` as
the `$GOPATH` in the container:

```bash
docker run --rm -it -v $GOPATH:/go \
    -w /go/src/github.com/sql-machine-learning/sqlflow \
    sqlflow:dev bash
```

Inside the Docker container, start a MySQL server in the background

```
service mysql start&
```

run all the tests as

```
go generate ./...
go install ./...
go test -v ./...
```

where `go generate` invokes the `protoc` command to translate `server/sqlflow.proto`
into `server/sqlflow.pb.go` and `go test -v` builds and run unit tests.


### Release

The above build process currently generates two binary files in
`$GOPATH/bin` on the host.  To package them into a Docker image,
please run

```bash
docker build -t sqlflow -f ./Dockerfile $GOPATH/bin
```

To publish the released Docker image to our official DockerHub
```bash
docker tag sqlflow sqlflow/sqlflow:latest
docker push sqlflow/sqlflow:latest
```

## Demo: Command line Prompt

The demo requires a MySQL server instance with populated data. If we don't, we could
follow [example/datasets/README.md](/example/datasets/README.md) to start one on the host.
After setting up MySQL, run the following inside the Docker container

```bash
go run cmd/demo/demo.go --db_user root --db_password root --db_address host.docker.internal:3306
```

You should be able to see the following prompt

```
sqlflow>
```
