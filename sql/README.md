# SQLFlow

## Build, Run, Test

Before running the unit tests, we need to build and run a Docker container that
hosts a MySQL database following [this guide](../example/datasets/README.md).

To build the parser using `goyacc` and run all unit tests, use the following
command:

```bash
go get -d ./... && goyacc -p sql -o parser.go sql.y && go test -v
```
