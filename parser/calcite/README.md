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

Generate Java source code from protobuf messages:

```bash
 docker run --rm -it -v $PWD:/work -w /work calcite:dev protoc --java_out=. CalciteParser.proto
 docker run --rm -it -v $PWD:/work -w /work calcite:dev protoc --grpc-java_out=. CalciteParser.proto
 ```

Build and generate `.class` files:

```bash
docker run --rm -it -v $PWD:/work -w /work calcite:dev javac *.java
```


All, actually, we can do all above in a single command:

```bash
docker run --rm -it -v $PWD:/work -w /work calcite:dev bash ./build_and_test.bash
```
