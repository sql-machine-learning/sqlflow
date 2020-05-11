# Build and Test Third Party Parsers

We build and test two third party parsers, Hive and Calcite, and wrap these two parsers into either a gRPC server or a command line.

## Build and Test the Parsers

To make sure that all developers use the same version and configuration of development tools like JDK and Maven, we install all of them into a Docker image `sqlflow/sqlflow:ci`.

To download the image, type the following command

```bash
docker pull sqlflow/sqlflow:ci
```

To start a Docker container that runs the above image, we can type the following command:

```bash
docker run --rm -it -v $HOME:/root -v $PWD:/work -w /work sqlflow/sqlflow:ci bash
```

Please be aware the `-v $HOME:/root` binds the `$HOME` directory on the host to the `/root`, the home directory, in the container, so that when we run Maven in the container, it saves the downloaded jars into `$HOME/.m2`.

In the container, we can type the following command to generate the gRPC related code.

```bash
protoc --java_out=src/main/java --grpc-java_out=src/main/java/ --proto_path=src/main/proto/ src/main/proto/parser.proto
```

In the container, we can type the following command to test the package

```bash
mvn test
```
