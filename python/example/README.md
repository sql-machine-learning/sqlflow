## Demo

Quick demo on scheduler: given json, produces trained model.

1. Start a MySQL Server as documented [here](https://github.com/wangkuiyi/sqlflow/blob/develop/doc/mysql-setup.md)

```bash
docker run --rm \
   -v /tmp/test1:/var/lib/mysql \
   --name mysql01 \
   -e MYSQL_ROOT_PASSWORD=root \
   -e MYSQL_ROOT_HOST='%' \
   -p 3306:3306 \
   -d \
   mysql/mysql-server:8.0
```

2. Build sqlflow docker image

```bash
docker build -t sqlflow ../..
```

3. Import data
```bash
docker run --rm \
    --network="host" \
    -v $PWD/../:/sqlflow \
    sqlflow \
    /bin/bash -c "cd /sqlflow/example && python load_data.py"
```

4. Train model
```bash
docker run --rm \
    --network="host" \
    -v /tmp/:/tmp \
    -v $PWD/../:/sqlflow \
    sqlflow \
    /bin/bash -c "cd /sqlflow && cat example/infer.json | python scheduler.py"
```

You should find the trained model at `/tmp/my_dnn_model`.

5. Evaluate model
```bash
docker run --rm \
    --network="host" \
    -v /tmp/:/tmp \
    -v $PWD/../:/sqlflow \
    sqlflow \
    /bin/bash -c "cd /sqlflow && cat example/train.json | python scheduler.py"
```

This would give

```text
Test set accuracy: 1.00000
```
