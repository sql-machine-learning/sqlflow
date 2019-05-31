# Running SQLFlow on Kubernetes

This is a tutorial on how to run SQLFlow on Kubernetes, this tutorial will deploy:
- MySQL server with the SQLFlow example datasets loaded, and
- SQLFlow components contains the sqlflow-server the Jupyter notebook.

Then you can run the SQLFlow query in the Jupyter notebook on your web browser.

## Preparation

1. Setup a Kubernetes cluster: You can refer to the [official page](https://kubernetes.io/docs/setup) to set up a 
full cluster or use a local quick start tool: [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/)
This tutorial would use [minikube] to demonstrate the SQLFlow.
1. [Install the kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/), which is the command line tool
to interact with the Kubernetes cluster.
1. Pull the official Docker image [sqlflow/sqlflow:latest](https://hub.docker.com/r/sqlflow/sqlflow) or [build
from source](/doc/build.md)

## Deploy the SQLFlow Components

1. Deploy the MySQL service and sqlflow service in the difference containers in one Pod. 
    ``` bash
    > kubectl create -f k8s/sqlflow-mysql.yaml
    ```

1. Testing your SQLFlow setup
    You can find a Pod on Kubernetes with the prefix `sqlflow-mysql-*`:
    ``` bash
    > kubectl get pods
    NAME    READY   STATUS    RESTARTS   AGE
    NAME                             READY   STATUS    RESTARTS   AGE
    sqlflow-mysql-77f8674899-dv269   2/2     Running   0          75m
    ```
    The logs of the two containers similar to:
    ``` bash
    > kubectl logs sqlflow-mysql-77f8674899-dv269 mysql
     * Starting MySQL database server mysqld
       ...done.
    > kubectl logs sqlflow-mysql-77f8674899-dv269 sqlflow
    Connect to the datasource mysql://root:root@tcp(10.100.73.238:3306)/?maxAllowedPacket=0
    2019/05/30 09:57:55 Server Started at :50051
    ```

## Running your Query in SQLFlow 

1. Copy the node IP of the sqlflow Pod on minikube as the follows command:
    ``` bash
    > minikube ip
    192.168.99.100
    ```
    **NOTE**: If you are using a **real** cluster, you can find the node domain/IP from the `NODE` colume
    using`kubectl get pods -o wide`:
    ``` bash
    > kubectl get pods -o wide
    NAME    READY   STATUS    RESTARTS   AGE   IP   NODE    NOMINATED   NODE    READINESS   GATES
    sqlflow-mysql-77f8674899-dv269  2/2    Running  0   9s  172.17.0.4  minikube   <none>           <none>
    ```

1. Open a web browser and go to '<node-ip>:8888', you can find the [SQLFlow example](/example/jupyter/example.ipynb) in the Jupyter notebook file lists.