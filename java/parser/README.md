# Build and Test Third Party Parsers

We build and test two third party parsers:

- HiveQL
- Calcite

## Build and Test the Parsers

### Docker

To make sure that all developers use the same version and configuration of development tools like JDK and Maven, we install all of them into a Docker image.

To build the image, type the following command:

```bash
docker build -t sqlflow:mvn .
```

The suffix `:mvn` refers to Maven.  We use Maven to build and run the tests in Java.


To start a Docker container that runs the above image, we can type the following command:

```bash
docker run --rm -it -v $HOME:/root -v $PWD:/work -w /work sqlflow:mvn bash
```

Please be aware the `-v $HOME:/root` binds the `$HOME` directory on the host to the `/root`, the home directory, in the container, so that when we run Maven in the container, it saves the downloaded jars into `$HOME/.m2`.

In the container, we can type the following command to test the package

```bash
$ mvn test
```

### IntelliJ

Import Project > Maven project.
