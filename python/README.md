# Python Code Template

To run a test, say, `test_sql_data.py`, we need to

```bash
docker run --rm -it -v $PWD:/work -w /work sqlflow/sqlflow service mysql start && python test_sql_data.py
```

where `service mysql start` will start a MySQL service inside the container with some sample data loaded.
