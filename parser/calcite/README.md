# CalciteParser gRPC Server for SQLFlow

## How to Build

The following build process doesn't require us to install Java SDK, Maven, protobuf-compiler, and any dependencies.  Instead, it installs all such staff into a Docker image and use the Docker image as the build toolchain.  Building in Docker containers standardizes development environments of all contributors, keeps our host computer clean, and works on macOS, Linux, BSD, Windows, and all platforms that have Docker.

Build the development Docker image:

```bash
docker build -t calcite:dev .
```

Or, if it takes too long time for you to build the image, please feel free to use mine:

```bash
docker pull cxwangyi/calcite:dev
docker tag cxwangyi/calcite:dev calcite:dev
```

We can build the server program using the Docker image:

```bash
docker run --rm -it -v $PWD:/work -w /work calcite:dev bash ./build_and_test.bash
```

The above command outputs `CalciteParserServer.class`.

## How to Run

We can run the server program using the above Docker image:

```bash
docker run --rm -d -p 50051:50051 -v $PWD:/work -w /work calcite:dev bash -c "java CalciteParserServer"
```
