# Build and Run Remote Parsers

Currently, we have two remote parsers, each as a gRPC server in Java:

- hiveql
- calcite


## Development Environment

To make sure that all developers use the same version and configuration of development tools like JDK, Maven, and protoc, as well as dependencies like gRPC, we install all of them into a Docker image.

To build the image, type the following command:

```bash
docker build -t sqlflow:mvn .
```

The suffix `:mvn` refers to Maven.  We use Maven to build and run the gRPC servers in Java.


## Build the Parsers

To start a Docker container that runs the above image, we can type the following command:

```bash
docker run --rm -it -v $HOME:/root -v $PWD:/work -w /work sqlflow:mvn bash
```

Please be aware the `-v $HOME:/root` binds the `$HOME` directory on the host to the `/root`, the home directory, in the container, so that when we run Maven in the container, it saves the downloaded jars into `$HOME/.m2`.

In the container, we can type the following command to build this package into a jar `target/hiveql-parser-1.0-SNAPSHOT.jar`.

```bash
$ mvn package
```

To build standalone jar, use:
```bash
$ mvn clean compile assembly:single
```
