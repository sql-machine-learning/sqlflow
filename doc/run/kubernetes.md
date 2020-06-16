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

## Setup SQLFlow Playground with Single-user Mode

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

1. Open a web browser and go to `http://localhost:8888`, and type in the token to login,
you can find many tutorials e.g. `iris-dnn.md` in the Jupyter Notebook file lists,
and do as what the tutorial says.

## Setup SQLFlow Playground with Multi-user Mode

From the above single-user mode section, SQLFLow playground use Jupyter Notebook as GUI system, and
Jupyter community provides [JupyterHub](https://jupyterhub.readthedocs.io/en/stable/) to serve Jupyter Notebook
for multiple users, [kubespawner] enable JupyterHub to spawan Jupyter Notebook server on Kubernetes. We packages
these components into a Docker image `sqlflow/sqlflow:jupyterhub`, you can check the [Dockerfile](/docker/jupyterhub/Dockerfile).

1. Run the following command to install SQLFlow JupyterHub and server.

    ``` bash
    kubectl apply -f https://raw.githubusercontent.com/sql-machine-learning/sqlflow/develop/doc/run/k8s/install-sqlflow-multi-users.yaml 
    ```

1. Monitor the installation using the following command until all components show a `Running` status.

    ``` bash
    kubectl get pods --watch
    ```

Congratulations! You have successfully installed SQLFlow playground with multi-user
mode on your Kubernetes cluster. Next you can login on the web page and run query on Jupyter Notebook server.

1. Map the Jupyter Notebook to a local port using the following command:

    ``` bash
    kubectl port-forward deployment/sqlflow-jupyterhub 8000:8000
    ```

1. Open your browser and go to `localhost:8000`, type in any username/password to login, wait a while, you would be in a 
Jupyter Notebook web page, and you can find many tutorials here.

Note: SQLFlow playground use [dummyauthenticator] as the authenticator, which allows all users login regardless of password,
you can find more authenticator method from [here](https://jupyterhub.readthedocs.io/en/stable/getting-started/authenticators-users-basics.html).
