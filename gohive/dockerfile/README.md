# Build and Test in Docker Containers

GoHive is a Hive driver for Go's database API.  To build and test it, we need not only the building tools but also Hive.  For the convenience of contributors, we install all tools into a Docker image so could we run and test in a Docker container.

The general usage is that we check out the source code on the host computer, then we start a Docker container and run building tools in the container.  The critical point is that we map the source code directory into the container.  Feel free to use any of your favorite editor, Emacs, Vim, Eclipse, installed and running on the host.

## Check out the Source Code

The following command

```bash
go get github.com/sql-machine-learning/sqlflow/gohive
```

clones GoHive to `$GOPATH/src/github.com/sql-machine-learning/sqlflow/gohive`.

## Build the Docker Image

The following command

```bash
docker build -t gohive:dev dockerfile
```

in the Dockerfile directory creates the Docker image `gohive:dev`.

## Build and Test in a Container

The following command starts a container and maps the `$GOPATH` directory on the host to the `/go` directory in the container.  Please be aware that the Dockerfile configures `/go` as the `$GOPATH` in the container.

```bash
docker run --rm -it -v $GOPATH:/go \
    -w /go/src/github.com/sql-machine-learning/sqlflow/gohive \
    gohive:dev bash
```

After the container prints many lines of logs showing that the Hive server is starting, we can build and run tests:

```
go build
go test -v
```
