# Install SQLFlow Playground on Your Kubernetes Cluster

This is a tutorial on how to install SQLFlow playground on your Kubernetes, this tutorial includes two sections:

- [Install SQLFlow playground with single-user mode](#install-sqlflow-with-single-user).
- [Install SQLFlow playground with multi-users mode](#install-sqlflow-with-multi-users).

Before starting any sections, please [Setup Minikube and Argo](#setup-minikube-and-argo) first.

## Setup Minikube and Argo

1. [install Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/) on your laptop.
1. Install Argo

    ``` bash
    kubectl create namespace argo
    kubectl apply -n argo -f https://raw.githubusercontent.com/argoproj/argo/v2.7.7/manifests/install.yaml
    ```

1. Access the Argo UI

    ``` bash
    kubectl -n argo port-forward deployment/argo-server 2746:2746
    ```

    Then visit: `http://127.0.0.1:2746`

## Install SQLFlow Playground with single-user Mode

On the single-user mode, we would install a MySQL server, a SQLFlow server with Jupyter Notebook as GUI on your Kubernetes cluster:

1. Run the following command to install SQLFlow and its dependencies:

    ``` bash
    kubectl apply -f https://raw.githubusercontent.com/sql-machine-learning/sqlflow/develop/doc/run/k8s/install-sqlflow.yaml
    ```

1. Monitor the installation using the following command until all components show a `Running` status and `3/3` ready.

    ``` bash
    kubectl get pods --watch
    ```

Congratulations! You have successfully installed SQLFlow with single-user
mode on your Kubernetes cluster. Next you can run your query using SQLFlow as the following command:

1. Retrieve the login token from logs

    ``` bash
    kubectl logs sqlflow-server notebook  | awk -F "token=" 'END{print $2}'
    ```

1. Map the Jupyter Notebook to a local port using the following command:

    ``` bash
    kubectl port-forward deployment/sqlflow-server 8888:8888
    ```

1. Open a web browser and go to `http://localhost:8888`, and type in the token to login, you can find many tutorials e.g. `iris-dnn.md` in the Jupyter Notebook file lists,
you can select one of them and do as what the tutorial says.

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

    **NOTE**: Should grant all the remote hosts can access to the MySQL server if you want to use the custom MySQL Docker image, the grant command like:
    ``` text 
    GRANT ALL PRIVILEGES ON *.* TO 'root'@'' IDENTIFIED BY 'root' WITH GRANT OPTION;
    ```

1. Check the SQLFlow Pods, you can find:
    - A MySQL Pod named `sqlflow-mysql-*`.
    - A JupyterHub Pod named `sqlflow-jhub-*`.
    - 3 SQLFlow gRPC server Pods named `sqlflow-server-*`, and it's easy to scale up/down the replica count by modifying the `replicas` field of the yaml file: [k8s/sqlflow-server.yaml](https://github.com/sql-machine-learning/sqlflow/tree/develop/doc/k8s/sqlflow-server.yaml).
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

JupyterHub using the PAMAuthenticator as the default authenticate method. the [PAM](https://en.wikipedia.org/wiki/Linux_PAM) can authenticate the system users with their username and password. You can find more information on [authenticators-users-basics](https://jupyterhub.readthedocs.io/en/stable/getting-started/authenticators-users-basics.html), and other authenticator methods from [here](https://github.com/jupyterhub/jupyterhub/wiki/Authenticators)

Next, please do as the following steps to create a user on the system and login on the Jupyterhub:

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
