# Running SQLFlow on Kubernetes

This is a tutorial on how to run SQLFlow on Kubernetes, this tutorial will deploy:
- MySQL server with the SQLFlow example datasets loaded, and
- SQLFlow component contains the sqlflow-server the Jupyter notebook.

Then you can run the SQLFlow query in the Jupyter notebook on the browser.

## Preparation

1. Kubernetes cluster: You can check the [offical page](https://kubernetes.io/docs/setup/) to set up a
Kubernetes cluster. This tutorial would use [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/)
which can the Kubernetes locally to demonstrate the SQLFlow.
1. [Install the kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/), which is the command line tool
to interact with the Kubernetes cluster.
1. Build the SQLFlow Docker image or using the official Docker image directly: [sqlflow/sqlflow:latest](https://hub.docker.com/r/sqlflow/sqlflow).

## Deploy the SQLFlow Components

1. Deploy the MySQL instance
    ``` bash
    > kubectl create -f mysql.yaml
    ```
1. Deploy the SQLFlow instance
    ``` bash
    > kubectl create -f sqlflow.yaml
    ```
1. Testing your SQLFlow setup
    You can find two Pods on Kubernetes
    ``` bash
    > kubectl get pods
    NAME    READY   STATUS    RESTARTS   AGE
    mysql-7554477fc5-k5tpr     1/1     Running   0          3h8m
    sqlflow-8555db67d8-4rcn4   1/1     Running   0          3h6m
    ```
    The logs of the two Pods similar to:
    ``` bash
    > kubectl logs mysql-7554477fc5-k5tpr
     * Starting MySQL database server mysqld
       ...done.
    > kubectl logs sqlflow-8555db67d8-4rcn4
    Connect to the datasource mysql://root:root@tcp(10.100.73.238:3306)/?maxAllowedPacket=0
    2019/05/30 09:57:55 Server Started at :50051
    ```

## Running your Query in SQLFlow 

1. Find the minikube IP as the follows command:
    ``` bash
    > minikube ip
    192.168.99.100
    ```
1. Open a web browser and go to '192.168.99.100:8888', you can find some [SQLFlow example](/example/jupyter/example.ipynb) in the Jupyter notebook. 