# Build from Source in a Docker Container

The source code of SQLFlow is in Go, Java, protobuf, yacc, and Python.  To build from source code, we need toolchains of all these languages.  In addition to that, we need to install MySQL, Hive, and MaxCompute client for unit tests.  To ease the software installation and configuration, we provide a `Dockerfile` that contains all the requirement software for building and testing.

## Prerequisite

1. Git for checking out the source code.
1. [Docker CE >= 18.x](https://docs.docker.com/docker-for-mac/install/) for building the Docker image of development tools.

## Checkout the Source Code

We can clone the source code to any working directory, say, `~/sqlflow`.

```bash
cd ~
git clone https://github.com/sql-machine-learning/sqlflow
```

## Build the Development Docker Image

We can build the Docker image from the `Dockerfile`.

```bash
cd sqlflow
docker build -t sqlflow .
```

Or, we can pull the Docker image [pre-built by the CI system](https://hub.docker.com/r/sqlflow/sqlflow/tags) from DockerHub.

```bash
docker pull sqlflow/sqlflow
docker tag sqlflow/sqlflow:latest sqlflow:latest
```

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
SQLFLOW_TEST_DB=mysql go test -v -p 1 ./...
```

The commandline `go generate` is necessary to call `protoc` for translating gRPC interface and to call `goyacc` for generating the parser.

The environment variable `SQLFLOW_TEST_DB=mysql` specify MySQL as the SQL engine during testing.  You can also choose `hive` for Apache Hive and `maxcompute` for Alibaba MaxCompute.

## Editing on Host

As the above `docker run` command binds the source code directory on the host computer to the container, we can edit the source code on the host using any editor, VS Code, Emacs, etc.

After the editing and before you can Git commit, please install the [`pre-commit`](https://pre-commit.com/) tool.  SQLFlow needs it to run pre-commit checks.

## The Command-line REPL

SQLFlow provides a command-line tool `repl` for evaluating SQL statements.  This tool makes it easy to debug.  To build it, run the following commands.

```bash
cd cmd/repl
go install
~/go/bin/repl --datasource="mysql://root:root@tcp(localhost:3306)/?maxAllowedPacket=0"
```

Please follow the [REPL tutorial](run/repl.md) to understand what we can do with the REPL.
