# Quick start

SQLFlow is currently under active development. For those are interested in trying
it out, we have provided a prompt demo. Play around with it. Any bug report and
issue are welcomed. :)

## Setup

### For users

1. Install Docker
1. Set up a MySQL server defined at `example/datasets`
1. Pull SQLFlow Docker image: `docker pull sqlflow/sqlflow:latest`

#### Run command line demo
1. `docker run -it --rm --net=host sqlflow/sqlflow:latest demo`

You should be able to see the following prompt

```
sqlflow>
```

#### Run Jupyter Notebook demo

```
docker run --rm -it -p 8888:8888 -v $PWD/example/jupyter:/tmp sqlflow/sqlflow:latest \
    bash -c "sqlflowserver --port 50051 --db_user root --db_password root --db_address localhost:3306 &
    SQLFLOW_SERVER=localhost:50051 jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root"
```

Open a Web browser and direct to `localhost:8888` and input the token. Then you
can create notebooks. In a cell, you should be able to type

```
%%sqlflow
select *
from iris.iris
limit 1
```

### For developers

- Install Go and Docker
- Get the source code of SQLFlow and all its dependencies
```bash
export GOPATH=/what/ever/directory
go get -insecure gitlab.alipay-inc.com/Arc/sqlflow
```
- Build the development Docker image. Even if go get builds and downloading source code,
we might still need this step, because if we run go get on macOS, the built SQLFlow
programs would be macOS binary files. However, to pack them into a Docker image,
we need Linux binary files.
```bash
cd $GOPATH/src/gitlab.alipay-inc.com/Arc/sqlflow
docker build -t sqlflow:dev -f Dockerfile.dev .
```
- Run the development Docker image to build SQLFlow
```bash
docker run --rm -it -v $GOPATH:/go -w /go/src/gitlab.alipay-inc.com/Arc/sqlflow sqlflow:dev
```
- Package the built SQLFlow binaries into the release Docker image
```bash
docker build -t sqlflow -f ./Dockerfile $GOPATH/bin
```
- Set up a MySQL server defined at `example/datasets`
- Start the demo: `docker run -it --rm --net=host sqlflow demo`

You should be able to see the following prompt

```
sqlflow> 
```

## Example

- Select data
```
sqlflow> select * from iris.iris limit 2;
-----------------------------
[6.4 2.8 5.6 2.2 2]
[5 2.3 3.3 1 1]
```
- Train a Tensorflow [DNNClassifier](https://www.tensorflow.org/api_docs/python/tf/estimator/DNNClassifier)
```
sqlflow> SELECT *
FROM iris.iris
TRAIN DNNClassifier
WITH n_classes = 3, hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO my_dnn_model;
-----------------------------
...
Training set accuracy: 0.96721
Done training
```
- Prediction using a trained model
```
sqlflow> SELECT *
FROM iris.iris
predict iris.predict.class
USING my_dnn_model;
```
