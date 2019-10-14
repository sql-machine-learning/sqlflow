# SQLFlow

## Build, Run, Test

Before running the unit tests, we need to pull the SQLFlow Docker image to
run as a build environment following [this guide](/doc/run/docker.md).

To build the parser using `goyacc` and run all unit tests, use the following
command:

```bash
go get -d ./... && goyacc -p sql -o parser.go sql.y && go test -v
```
