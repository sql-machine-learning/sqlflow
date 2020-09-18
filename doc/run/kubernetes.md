# Install SQLFlow Playground on Your Kubernetes Cluster

This is a tutorial on how to install SQLFlow playground on your Kubernetes, this tutorial includes two sections:

- [Setup SQLFlow playground with single-user mode](#setup-sqlflow-playground-with-single-user-mode).
- [Setup SQLFlow playground with multi-users mode](#setup-sqlflow-playground-with-multi-user-mode).

Before starting any sections, please [Setup Minikube and Argo](#setup-minikube-and-argo) first.

## Setup Minikube and Argo

1. [install Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/) on your laptop.
1. Install Argo

    ``` bash
    kubectl create namespace argo

    kubectl create rolebinding default-admin --clusterrole=admin --serviceaccount=default:default

    kubectl apply -n argo -f https://raw.githubusercontent.com/argoproj/argo/v2.7.7/manifests/install.yaml
    ```
1. Wait until Argo is up, run below command until you see all pods in argo namespace is `READY` and `Running`
    ```bash
    kubectl get pods -nargo --watch
    ```

1. Access the Argo UI

    ``` bash
    nohup kubectl -n argo port-forward deployment/argo-server 9001:2746 --address=0.0.0.0 &
    ```

    Then visit: `http://127.0.0.1:9001`

## Setup SQLFlow Playground with Single-user Mode

On the single-user mode, we would install a MySQL server,
a model zoo server and a SQLFlow server with Jupyter Notebook
as GUI on your Kubernetes cluster:

1. Run the following command to install SQLFlow and its dependencies:

    ``` bash
    kubectl apply -f https://raw.githubusercontent.com/sql-machine-learning/sqlflow/develop/doc/run/k8s/install-sqlflow.yaml
    ```

1. Monitor the installation using the following command until all components show a `Running` status and `4/4` ready.

    ``` bash
    kubectl get pods --watch
    ```

Congratulations! You have successfully installed SQLFlow with single-user
mode on your Kubernetes cluster. Next, you can try some tutorials with
following steps:

1. Map the Jupyter Notebook to a local port using the following command:

    ``` bash
    nohup kubectl port-forward pod/sqlflow-server 8888:8888 --address=0.0.0.0 &
    ```

1. Open a web browser and go to `http://localhost:8888`, you can find many
tutorials, e.g. `iris-dnn.npynb` in the Jupyter Notebook file lists, and do
as what the tutorial says.

## Setup SQLFlow Playground with Multi-user Mode

In above section, SQLFLow playground use Jupyter Notebook as GUI system
to serve a single user. Actually, Jupyter community provides the [JupyterHub](https://jupyterhub.readthedocs.io/en/stable/)
which can serve Jupyter Notebook for multiple users. [kubespawner](https://github.com/jupyterhub/kubespawner)
enable JupyterHub to spawan Jupyter Notebook server on Kubernetes. We packages
these components into a Docker image `sqlflow/sqlflow:jupyterhub`, you can check
the [Dockerfile](/docker/jupyterhub/Dockerfile).

Jupyterhub is someway suitable a team, so we have to consider about using `https` and add `authentication`.
Of course, you can choose not to enable them. In below section, we will describe how to install our
playground with your own `https` and `authentication` enabled/disabled.

1. Download k8s config file for our multi-user playground

    ``` bash
    wget https://raw.githubusercontent.com/sql-machine-learning/sqlflow/develop/doc/run/k8s/install-sqlflow-multi-users.yaml 
    ```
1. Prepare for `https`, we make a directory for keeping `ssl` certificate file,
    if you want to put it in other directory, make sure to modify the downloaded
    config file as well. Then put the ssl certificate files into this directory.

    ```bash
    mkdir /jupyter/certs
    cp {your_cert.key} /jupyter/certs/playground.sqlflow.tech.key
    cp {your_cert.pem} /jupyter/certs/playground.sqlflow.tech.pem
    ```

    **NOTE:** If you don't want to enable `https`, you can skip the certificate setup.
    Correspondingly, you need to set `SQLFLOW_JUPYTER_SSL_KEY` and
    `SQLFLOW_JUPYTER_SSL_CERT` to empty in config.

    ```yaml
    - name: SQLFLOW_JUPYTER_SSL_KEY
      value: ""
    - name: SQLFLOW_JUPYTER_SSL_CERT
      value: ""
    ```

1. Prepare for `authentication`. JupyterHub support kinds of
    [authentications](https://jupyterhub.readthedocs.io/en/stable/reference/authenticators.html).
    In our playground, we defaultly enabled the [GitHub OAuth](https://oauthenticator.readthedocs.io/en/latest/getting-started.html#github-setup). If you want to use `GitHub OAuth`, you may register
    your own [GitHub app](https://github.com/settings/applications/new) and
    get your `client_id` and `client_secret`. Then store these two information
    in kubernetes secret store (please keep them safe at anytime).

    ```
    kubectl create secret generic sqlflow \
        --from-literal=jupyter_oauth_client_id={client_id} \
        --from-literal=jupyter_oauth_client_secret={client_secret}
    ```

    **NOTE:** If you do not want any `authentication`, you may safely skip above step.
    **But** you still need to make a fake secret in k8s' store. Then disable
    OAuth in config by setting `SQLFLOW_JUPYTER_USE_GITHUB_OAUTH`'s value to 'false'.
    ```bash
    kubectl create secret generic sqlflow \
        --from-literal=jupyter_oauth_client_id=dummy \
        --from-literal=jupyter_oauth_client_secret=dummy
    ```
    Change config file to disable authentication.
    ```yaml
    - name: SQLFLOW_JUPYTER_USE_GITHUB_OAUTH
      value: "false"
    ```

1. Next is to deploy the cluster using below command.
    ```bash
    kubectl apply -f install-sqlflow-multi-users.yaml
    ```

1. Monitor the installation using the following command until all components show a `Running` status.

    ``` bash
    kubectl get pods --watch
    ```

Congratulations! You have successfully installed SQLFlow playground with multi-user
mode on your Kubernetes cluster. Next you can login on the web page and run query on Jupyter Notebook server.

1. Map the Jupyter Notebook to a local port using the following command:

    ``` bash
    nohup kubectl port-forward deployment/sqlflow-jupyterhub 8000:8000 --address=0.0.0.0 &
    ```

1. Open your browser and go to `localhost:8000`
1. If you use `GitHub OAuth`, then click the auth button to login.
    If you disabled the authentication, just type in any username/password
    to login. Wait a while, you would enter a Jupyter Notebook web page,
    and you can find many tutorials here.

## Trouble Shooting

1. Sometimes, you can't get the GitHub files with wget, like `https://raw.githubusercontent.com/argoproj/argo/v2.7.7/manifests/install.yaml`.

    Try open it in your browser and save to local, or use a VPN to get the file.

1. Port-forwarding may lost function after a while of idle time.

    Just kill them and re-forwarding.

1. First time run Jupyter Notebook, reporting databse is not found.

    This may because you login and enter the notebook too fast, the MySQL
    service is not ready yet.

1. Pulling docker image is really slow.

    You may change your docker repository. Or pull the images to local beforehand.

1. Update my playground.

    By default, SQLFlow playground will use local cached images to work. You can
    delete and re-pull the images to update the playground. You may mostly want
    to update those images with prefix `sqlflow/sqlflow:`