# Running SQLFlow on Kubernetes with Google Cloud SQL

This tutorial is based on the original [Running SQLFlow on
Kubernetes](/doc/run_on_kubernetes.md) tutorial, but modified to use a MySQL
instance hosted on Google Cloud SQL service. This tutorial will deploy:
- A Google Cloud SQL instance with some example data loaded,
- The SQLFlow gRPC server, and
- The Jupyter Notebook server with SQLFlow magic command installed.

Then you can run the SQLFlow query in the Jupyter notebook on your web browser.

## Prerequisites

1. Setup a Google Cloud Platform project with billing enabled: You can refer to
[this link](https://cloud.google.com/resource-manager/docs/creating-managing-projects) to create a Google cloud
project, and [this link](https://cloud.google.com/billing/docs/how-to/modify-project) to enable billing
for the project.
1. Setup a Kubernetes cluster: You can refer to the [official page](https://kubernetes.io/docs/setup) to set up a
full cluster or use a local quick start tool: [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/).
1. [Install kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/), which is the command line tool
to interact with the Kubernetes cluster.
1. Make sure the Kubernetes nodes can pull the official SQLFlow Docker image [sqlflow/sqlflow:latest] or your [custom
Docker image](/doc/build.md).

## Set up the Google Cloud SQL instance.

Here is the [official quickstart page](https://cloud.google.com/sql/docs/mysql/quickstart) for Google Cloud SQL.

1. Follow the instruction in the official quickstart page to create a Cloud SQL
   instance. Note that
   * Please choose the **Second Generation**, which supports secure access from
     Cloud SQL Proxy.
   * This demo assumes the user/password for the MySQL instance to be **root**/**root**.

1. There are multiple ways of connecting to Cloud SQL instance from applications
   (in our case, the sqlflow server). We choose to use the [Cloud SQL Proxy docker image](https://cloud.google.com/sql/docs/mysql/sql-proxy).
   1. [Enable the Cloud SQL Admin API](https://console.cloud.google.com/flows/enableapi?apiid=sqladmin).
   1. Follow this [link](https://cloud.google.com/sql/docs/mysql/connect-external-app#4_if_required_by_your_authentication_method_create_a_service_account)
      to create a service account for authentication with the Cloud SQL instance and download the private
      key json file at <span style="color:red">*PATH-TO-KEY-FILE*</span>.json.
   1. Import the service account credential to your Kubernetes clusters as a
      Secret.
      ``` bash
      > kubectl create secret generic cloud-service-key --from-file=key.json=PATH-TO-KEY-FILE.json
      ```

## Deploy the SQLFlow Components

1. Replace the '<INSTANCE_CONNECTION_NAME>' in k8s/sqlflow-cloudsql.yaml with
   the real Cloud SQL instance full name. Note that this include the GCP
   project, the region of your Cloud SQL instance and the name of the Cloud SQL
   instance.

1. Deploy the SQLFlow Pod on Kubernetes
    ``` bash
    > kubectl apply -f k8s/sqlflow-cloudsql.yaml
    ```
    The above command starts a Pod with four containers
    1. A container running the `sqlflow/sqlflow:latest` to populate some example data
       into the Cloud SQL instance.
    1. A Cloud SQL Proxy container redirecting the local mysql queries/commands to the
       Cloud SQL instance.
    1. Two containers running the `sqlflow/sqlflow:latest` Docker image,
       one container running the SQLFlow gRPC server and the other running Jupyter
       Notebook server. You can also use your custom Docker image by editing the `image`
       fields of the yaml file [k8s/sqlflow-mysql.yaml](/doc/k8s/sqlflow-cloudsql.yaml):
    ``` yaml
    spec:
        ...
        containers:
        - image: <your repo name>/sqlflow:latest
    ```

1. Testing your SQLFlow setup
    You can find a Pod on Kubernetes with the prefix `sqlflow-cloudsql-*`:
    ``` bash
    > kubectl get pods
    NAME                                READY   STATUS    RESTARTS   AGE
    sqlflow-cloudsql-7dfd6b578c-2d6qs   4/4     Running   0          1h
    ```

1. Expose the deployment with a service
    ``` bash
    > kubectl expose deployment sqlflow-cloudsql --type=LoadBalancer --name=sqlflow-service
    ```
    You can check the public IP address for the service:
    ``` bash
    > kubectl get services
    NAME               TYPE           CLUSTER-IP   EXTERNAL-IP       PORT(S)
    sqlflow-service    LoadBalancer   10.72.4.49   35.123.254.123    8888:30777/TCP
    ```

## Running your Query in SQLFlow

1. Open a web browser and go to '*EXPERNAL-IP*:8888', you can find the [SQLFlow example](/example/jupyter/example.ipynb) in the Jupyter notebook file lists.
