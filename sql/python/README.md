# Python Code Template

To run a test, say, `test_sql_data.py`, we need to

1. Start a container that runs MySQL server with populated data following [this guide](https://github.com/sql-machine-learning/sqlflow/blob/develop/example/datasets/README.md).

1. Run the tests in a SQLFlow container that has TensorFlow and `mysql_connector` installed:

   ```bash
   docker run --rm -it --network="host" -v $PWD:/work -w /work sqlflow/sqlflow python test_sql_data.py
   ```

   where `--network="host"` allows processes running in the container to access the host's network, where the MySQL server container exposes its port.
