# CalciteParser gRPC Server for SQLFlow

## Develop using Docker

We provide a Dockerfile that installs all Java tools and dependencies into a Docker image, which eases the build and run.  The following command builds the development Docker image.

```bash
docker build -t calcite:dev .
```

If it takes too long time for you to build the image, please feel free to use the pre-built one on DockerHub.com.

```bash
docker pull cxwangyi/calcite:dev
docker tag cxwangyi/calcite:dev calcite:dev
```

## Build and Run

The following command builds `CalciteParserServer.class` and runs it:

```bash
docker run --rm -d -p 50051:50051 -v $PWD:/work -w /work calcite:dev bash ./build_and_run.bash
```

## Test

Given an instance running in Docker container, we can run the following command to test it.

```bash
go generate
SQLFLOW_CALCITE_PARSER=127.0.0.1:50051 go test -v
```
