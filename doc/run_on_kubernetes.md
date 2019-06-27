# Running SQLFlow on Kubernetes

This is a tutorial on how to run SQLFlow on Kubernetes, and this tutorial will deploy:
- A MySQL server instance with some example data loaded,
- The SQLFlow gRPC server, and 
- The Jupyter Notebook server with SQLFlow magic command installed.
- The JupyterHub which can serve multiple Notebook server for various users.

There are two sections in this tutorial:

- [Deploy the All-in-One SQLFlow](#deploy-the-sqlflow-all-in-one) deployed the SQLFlow on Kubernetes quickly.
- [Deploy the SQLFlow Hub](#deploy-the-sqlflow-hub) deployed an SQLFlow cluster and a JupyterHub server which can serve Notebook server instances for users.

## Prerequisites

1. Setup a Kubernetes cluster: You can refer to the [official page](https://kubernetes.io/docs/setup) to set up a 
full cluster or use a local quick start tool: [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/)
This tutorial would use [minikube] to demonstrate the SQLFlow.
1. [Install kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/), which is the command line tool
to interact with the Kubernetes cluster.
1. Make sure the Kubernetes nodes can pull the official SQLFlow Docker image [sqlflow/sqlflow:latest] or your [custom
Docker image](/doc/build.md).

## Deploy the All-in-One SQLFlow

1. Deploy the SQLFlow Pod on Kubernetes
    ``` bash
    > kubectl create -f k8s/sqlflow-mysql.yaml
    ```
    The above command deploys a Pod, a MySQL server instance, a SQLFlow gRPC server and the Jupyter Notebook server runs in this Pod. You can also use
    your custom Docker image by editting the `image` field of the yaml file: [k8s/sqlflow-all-in-one.yaml](/doc/k8s/sqlflow-all-in-one.yaml)
    ``` yaml
    spec:
        ...
        containers:
        - image: <your repo name>/sqlflow:latest
    ```

1. Testing your SQLFlow setup
    You can find a Pod on Kubernetes which name is `sqlflow-all-in-one-<POD-ID>`:
    ``` bash
    > kubectl get pods
    NAME    READY   STATUS    RESTARTS   AGE
    NAME                             READY   STATUS    RESTARTS   AGE
    sqlflow-all-in-one-9b57566c9-8xkpk   1/1     Running   0          60s
    ```
    The logs of the Pod similar to:

### Running your Query in SQLFlow 

1. Copy the node IP of the sqlflow Pod on minikube as the follows command:
    ``` bash
    > minikube ip
    192.168.99.100
    ```
    **NOTE**: If you are using a **real** cluster, you can find the node domain/IP from the `NODE` column
    using`kubectl get pods -o wide`:
    ``` bash
    > kubectl get pods -o wide
    NAME                                 READY   STATUS    RESTARTS   AGE     IP           NODE       NOMINATED NODE   READINESS GATES
    sqlflow-all-in-one-9b57566c9-8xkpk   1/1     Running   0          24s     172.17.0.9   minikube   <none>           <none>
    ```

1. Open a web browser and go to '<node-ip>:8888', you can find the [SQLFlow example](/example/jupyter/example.ipynb) in the Jupyter notebook file lists.

## Deploy the SQLFlow Hub

This section will deploys
SQLFlow Hub using JupyterHub to serve Jupyter notebook for multiple users, 
and easy to scale up/down the SQLFlow gRPC server according to workload.

1. Build the SQLFlow Hub Docker image and push to a registry server that the Kubernetes Note can access it.
    ``` bash
    $ cd k8s/sqlflowhub
    $ docker build -t <your-repo>/sqlflowhub .
    $ docker push <your-repo>/sqlflowhub
    ```

1. Deploy the MySQL, SQLFlow gRPC server and JupyterHub step by step:

    ``` bash
    kubectl create -f k8s/sqlflow-mysql.yaml
    kubectl create -f k8s/sqlflow-server.yaml
    kubectl create -f k8s/sqlflow-jhub.yaml
    ```

1. Check the SQLFlow Pods, you can find:
    - A MySQL Pod named `sqlflow-mysql-*`.
    - A JupyterHub Pod named `sqlflow-jhub-*`.
    - 3 SQLFlow gRPC server Pods named `sqlflow-server-*`, and it's easy to scale up/down the replica count by modifying the `replicas` field of the yaml file: [k8s/sqlflow-server.yaml](/doc/k8s/sqlflow-server.yaml).
    ``` bash
    $ kubectl get pods
    NAME                              READY   STATUS    RESTARTS   AGE
    sqlflow-jhub-78f96dcf88-sbvt6     1/1     Running   0          4m13s
    sqlflow-mysql-55db79fd98-nhfjp    1/1     Running   0          4h8m
    sqlflow-server-7444b4466d-frbcn   1/1     Running   0          4h
    sqlflow-server-7444b4466d-h5w9c   1/1     Running   0          4h
    sqlflow-server-7444b4466d-kndwx   1/1     Running   0          4h
    ```

1.  Check the SQLFlow Service, so the Notebook server can connect them across their ClusterIP and Port:
    - A MySQL Service named `sqlflow-mysql`, and
    - An SQLFlow server Service named `sqlflow-server`

    ``` bash
    $ kubectl get svc
    NAME             TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)     AGE
    kubernetes       ClusterIP   10.96.0.1        <none>        443/TCP     29d
    sqlflow-mysql    ClusterIP   10.102.193.217   <none>        3306/TCP    6h4m
    sqlflow-server   ClusterIP   10.102.65.39     <none>        50051/TCP   5h56m
    ```

### Login the JupyterHub

JupyterHub using the PAMAuthenticator as the default authenticate method. the PAM can authenticate the system users with their username and password. You can find more information on [authenticators-users-basics](https://jupyterhub.readthedocs.io/en/stable/getting-started/authenticators-users-basics.html), and other authenticator methods from [here](https://github.com/jupyterhub/jupyterhub/wiki/Authenticators)

Next, please do as the following steps to create a user on the system and log in the Jupyterhub:

1. List the Pods and execute into the `sqlflow-jhub` Pod
    ``` bash
    $ kubectl get po
    NAME                              READY   STATUS    RESTARTS   AGE
    sqlflow-jhub-78f96dcf88-gp8dg     1/1     Running   0          26m
    sqlflow-mysql-55db79fd98-nhfjp    1/1     Running   0          51m
    ...
    $ kubectl exec -it sqlflow-jhub-78f96dcf88-gp8dg bash 
    ```

1. Create a user and set a password by the `adduser` command:
    ``` bash
    $ adduser sqlflow -q --gecos "" --home /home/sqlflow
    Enter new UNIX password:
    Retype new UNIX password:
    passwd: password updated successfully
    ```

1. Open your browser and go to `<node-ip>:8000` and log in by the username/password as the above step. If you passed the authenticator, the JupyterHub would launch a Notebook server for your account, and then you can run your SQLFlow query in it.
