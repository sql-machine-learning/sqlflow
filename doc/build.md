# Build from Source in a Docker Container

The source code of SQLFlow is in Go, Java, protobuf, yacc, and Python.
To build from source code, we need toolchains of all these languages.
In addition to that, we need to install MySQL, Hive, and MaxCompute
client for unit tests.  To ease the software installation and
configuration, we provide a `Dockerfile` that contains all the
requirement software for building and testing.

## Prerequisite

1. Git for checking out the source code.
1. [Docker CE >= 18.x](https://docs.docker.com/docker-for-mac/install/) for
   building the Docker image of development tools.

## Checkout the Source Code

We can clone the source code to any working directory, say, `~/sqlflow`.

```bash
cd ~
git clone https://github.com/sql-machine-learning/sqlflow
```

## Build from Source Code

To standardize the building process, we define the development
environment as a Docker image `sqfllow:dev` in
`/docker/dev/Dockerfile`.  To make it easy to deploy SQLFlow, we
release the building result as a Docker image `sqlflow:ci`.  Please
follow [these steps](../docker/dev/README.md) in to bulid
`sqlflow:dev` and then `sqlflow.ci`.  You can also use the prebuilt
images on DockerHub.com.

## Build and Test

Let us start a container running the development Docker image.

```bash
docker run --rm -it -v $HOME/sqlflow:/sqlflow -w /sqlflow sqlflow bash
```

In the Docker container, we need to start a MySQL server for testing.

```bash
service mysql start
```

Then, we can build and run tests.

```bash
go generate ./...
PYTHONPATH=/sqlflow/python SQLFLOW_TEST_DB=mysql gotest -v -p 1 ./...
```

The commandline `go generate` is necessary to call `protoc` for translating gRPC interface and to call `goyacc` for generating the parser.

The environment variable `PYTHONPATH=$GOPATH/src/sqlflow.org/sqlflow/python` ensures the python part of SQLFlow in the Docker image is up to date.

The environment variable `SQLFLOW_TEST_DB=mysql` specify MySQL as the SQL engine during testing.  You can also choose `hive` for Apache Hive and `maxcompute` for Alibaba MaxCompute.

The command `gotest` with `-p 1` argument is necessary to run all tests, otherwise you will encounter the same problem as this [`issue`](https://github.com/sql-machine-learning/sqlflow/issues/1283).   Please feel free to use `go test` instead of `gotest`.  We use the latter one for colorized output.

## Editing on Host

As the above `docker run` command binds the source code directory on the host computer to the container, we can edit the source code on the host using any editor, VS Code, Emacs, etc.

After the editing and before you can Git commit, please install the [`pre-commit`](https://pre-commit.com/) tool.  SQLFlow needs it to run pre-commit checks.

## The Command-line Tool

SQLFlow provides a command-line tool `sqlflow` for evaluating SQL statements.  This tool makes it easy to debug.  To build it, run the following commands.

```bash
cd cmd/sqlflow
go install
docker run -d --rm -P -p 50051 --name sqlflowserver \
    sqlflow/sqlflow bash -c "/start.sh sqlflow-server-with-dataset"
~/go/bin/sqlflow --sqlflow_server="$(docker port sqlflowserver 50051)" \
 --datasource="mysql://root:root@tcp(localhost:3306)/?maxAllowedPacket=0"
```

Please follow the [command-line tool tutorial](run/cli.md) to understand what we can do with the tool.
