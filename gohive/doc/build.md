# Canonical Development Environment

Referring to [this example](https://github.com/wangkuiyi/canonicalize-go-python-grpc-dev-env),
we create a canonical development environment for Go programmers using Docker.

### Editing on Host

When we use this Docker image for daily development work, the source code relies
on the host computer instead of the container. The source code includes this repo
and all its dependencies, for example, the Go package `google.golang.org/grpc`.
Code-on-the-host allows us to run our favorite editors (Emacs, VIM, Eclipse, and more)
on the host.  Please free to rely on editors add-ons to analyze the source code
for auto-completion.

### Building in Container

We build a Docker image that contains development tools:

1. The Go compiler
1. The standalone Hive server

Because this repo contains Go code, please make sure that you have the directory
structure required by Go. On my laptop computer, I have

```bash
export GOPATH=$HOME/go
```

You could have your `$GOPATH` pointing to any directory you like.

Given `$GOPATH` set, we could git clone the source code of our project by running:

```bash
go get github.com/wangkuiyi/sqlflow/gohive
```

Change the directory to our project root and build the Docker image:

```bash
cd $GOPATH/src/github.com/wangkuiyi/sqlflow/gohive
docker build -t gohive dockerfile/all_in_one
```

## How to Build and Test

We build and test the project inside the docker container.

To run the container, we need to map the `$GOPATH` directory on the host into the
`/go` directory in the container, because the Dockerfile configures `/go` as
the `$GOPATH` in the container:

```bash
docker run --rm -it -v $GOPATH:/go \
    -w /go/src/github.com/wangkuiyi/sqlflow/gohive
    gohive bash
```

You should be able to see a series of logs because the gohive image is starting a Hive server.

Then build and run all the tests as

```
go build
go test -v
```