# Python Code Template

To run all the tests with MySQL as data source

1. Start our development environment mentioned in [build.md](/doc/build.md)
   ```bash
   docker run --rm -it -v $GOPATH:/go \
       -w /go/src/github.com/sql-machine-learning/sqlflow/sql/python \
       sqlflow/sqlflow:latest bash
   ```

2. In side the container, start MySQL server via `service mysql start`. Then run
all the tests via
   ```bash
   SQLFLOW_TEST_DB=mysql python -m unittest discover -v "*_test.py"
   ```
