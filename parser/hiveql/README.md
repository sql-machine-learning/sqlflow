# hiveql-parser

This package is inspired by https://github.com/qzchenwl/hiveql-parser.

## Development Environment

To make sure that all developers are using the same version of JDK and Maven, we install these tools into a Docker image.

```bash
cd $PWD
docker build -t sqlflow:hiveql .
```

## Build

Then we can start a Docker container that runs the Docker image.

```bash
docker run --rm -it -v $HOME:/root -v $PWD:/work -w /work sqlflow:hiveql bash
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

## Run

```bash
$ javar -jar /path/to/hiveql-parser.jar /path/to/your-code.sql
```

## Examples

```bash
$ java -jar hiveql-parser.jar <(echo "select count(*) as count, myfield from &0rz") 2>/dev/null
[1,39]: line 1:39 cannot recognize input near '&' '0rz' '<EOF>' in join source
```
