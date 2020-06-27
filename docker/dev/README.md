# The DevBox Docker Image

The Dockerfile in this directory defines the devbox Docker image of
SQLFlow.  This Docker image includes the build toolchain of Go,
Python, protobuf, etc.  It doesn't change with respect to the source
code of SQLFlow.

## Build the DevBox Image

In the root directory of this project, run the following command.

```bash
docker build -t sqlflow:dev -f docker/dev/Dockerfile .
```

## Build SQLFlow from Source Code

To build SQLFlow, we need to bind mount the whole source tree on the
host into the container running the devbox Docker image.  To make it
possible, we need to change to the root directory of the source tree.

```bash
cd $(git rev-parse --show-toplevel)
```

The following command bind mounts the source tree into the container
and builds SQLFlow.  The generated binary executables are put in
sub-directory `build`.  This output directory then can be used to build
sqlflow:server, sqlflow:jupyter, etc. Please be aware that it also
bind mounts `$HOME/.m2`, the Maven local repository, into the container
to reuse Maven cache and reduce the building time of Java code.

```bash
docker run --rm -it \
  -v $GOPATH:/root/go \
  -v $HOME/.m2:/root/.m2 \
  -v $HOME/.cache:/root/.cache \
  -v $PWD:/work -w /work \
  sqlflow:dev
```

