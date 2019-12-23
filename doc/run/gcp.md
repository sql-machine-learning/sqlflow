# Running SQLFlow on Google Cloud Platform

This tutorial introduces running SQLFlow on Google Kubernetes Engine with
Google CloudSQL service. It is built on top of the knowledge from the [Running SQLFlow on Kubernetes](kubernetes.md)
tutorial, so make sure to check it out first.

This tutorial will walk you through the steps of:
- Setting up a Google CloudSQL MySQL instance with necessary Google Cloud VPC (Virtual
  Private Cloud) setup to access the CloudSQL instance with private IP address.
- Setting up a Google Kubernetes Engine cluster to access CloudSQL MySQL Instance.
- Launch SQLFlow demo (SQLFlow gRPC server and Jupyter Notebook server) on GKE cluster.

## Prerequisites

1. You need to have a valid Google Cloud Platform project ([instruction for creation](https://medium.com/google-cloud/how-to-create-cloud-platform-projects-using-the-google-cloud-platform-console-e6f2cb95b467))
   with [billing enabled](https://cloud.google.com/billing/docs/how-to/manage-billing-account).
1. For your GPC project, you need to have [Kubernetes Engine API](https://console.cloud.google.com/apis/library/container.googleapis.com?q=Kubernetes),
   [Cloud SQL Admin API](https://console.cloud.google.com/apis/library/sqladmin.googleapis.com?q=Cloud%20SQL%20Admin)
   and [Service Networking API](https://console.cloud.google.com/apis/library/servicenetworking.googleapis.com?q=Service%20Networking)
   enabled.
1. [Install gcloud](https://cloud.google.com/sdk/gcloud/), which is the command-line interface for interacting with Google Cloud Platform.
1. [Install kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/), which is the command-line interface for interacting with the Kubernetes cluster. Note that this tutorial will create the Kubernetes cluster with the default version documented in the [GKE Versioning and Upgrades](https://cloud.google.com/kubernetes-engine/docs/how-to/creating-a-cluster). You need to make sure the installed kubectl version works well with it. You can also use the following command to install the correct kubectl.
    ``` bash
    > gcloud components install kubectl
    ```

## Set Up gcloud CLI

1. Authenticate gcloud to access the Google Cloud Platform with Google user
   credentials
    ``` bash
    > gcloud auth login ${YOUR_GCP_ACCOUNT_USER}
    ```
1. Set gcloud project configuration
    ``` bash
    > gcloud config set project ${YOUR_GCP_PROJECT_ID}
    ```

## Set Up Google CloudSQL MySQL Instance

1. Create a VPC (Virtual Private Cloud) on Google Cloud Platform. [VPC](https://cloud.google.com/vpc/docs/overview)
   provides a logically isolated network for your GKE clusters to communicate
   with services like CloudSQL, so traffic between SQLFlow server and CloudSQL
   MySQL instance does not go through public Internet.
    ``` bash
    > gcloud compute networks create sqlflow-demo-vpc --subnet-mode custom
    ```

1. Reserve an ip range for the created VPC. This ip range will be used for private service
   connections from applications (in our case, SQLFlow server) running within the created VPC
   to communicate with CloudSQL MySQL instance.
    ``` bash
    > gcloud beta compute addresses create vpc-peering-sqlflow-demo-vpc --global --purpose VPC_PEERING --description="For sqlflow with cloudsql private connection" --addresses 10.20.0.0 --prefix-length 16 --network sqlflow-demo-vpc
    ```

1. Create a private service connection between the created VPC and Google API services.
    ``` bash
    > gcloud services vpc-peerings connect --service servicenetworking.googleapis.com --ranges vpc-peering-sqlflow-demo-vpc --network sqlflow-demo-vpc
    ```

    You can read more about private services access [here](https://cloud.google.com/vpc/docs/configure-private-services-access).
    To summary, this private connection establishes a VPC Network Peering
    connection between our VPC (in which our application will be running), and
    the service producer's VPC network (the CloudSQL MySQL instance we will
    create will be running within a different VPC network managed by GCP
    internally).

1. Create a CloudSQL MySQL instance.
    ``` bash
    > gcloud beta sql instances create sqlflow-cloudsql-instance --network sqlflow-demo-vpc --no-assign-ip --tier db-n1-standard-2 --region us-central1
    Creating Cloud SQL instance...done.
    NAME                       DATABASE_VERSION  LOCATION       TIER              PRIMARY_ADDRESS  PRIVATE_ADDRESS  STATUS
    sqlflow-cloudsql-instance  MYSQL_5_7         us-central1-a  db-n1-standard-2  -                10.20.0.3        RUNNABLE
    ```

    You can find the *Instance connection name* and *Private IP address* for the
    newly created CloudSQL MySQL instance at the [CloudSQL console](https://console.cloud.google.com/sql/instances/sqlflow-cloudsql-instance/overview).
    Note that
    - the private ip address is within the IP range we reserved in the previous step.
    - You should be able to see a private connection between your VPC and the
      newly created CloudSQL MySQL instance on the VPC console.

1. Configure the default CloudSQL MySQL user account password
   For this demo purpose, we assume the *root* user of our CloudSQL MySQL
   instance has password *root*. To configure this, run the following:
    ``` bash
    > gcloud sql users set-password root --host=% --instance=sqlflow-cloudsql-instance --password=root
    ```


## Set Up a Kubernetes Cluster on Google Kubernetes Engine

1. Create a VPC subnet to be used by our Kubernetes cluster on GKE.
    ``` bash
    > gcloud compute networks subnets create sqlflow-demo-subnet --network sqlflow-demo-vpc --region us-central1 --range 10.10.0.0/16
    NAME                 REGION       NETWORK           RANGE
    sqlflow-demo-subnet  us-central1  sqlflow-demo-vpc  10.10.0.0/16
    ```

1. Create a Kubernetes cluster on GKE.
    ``` bash
    > gcloud container clusters create sqlflow-gke-cluster --zone us-central1-a --num-nodes 3 --network sqlflow-demo-vpc --subnetwork sqlflow-demo-subnet
    kubeconfig entry generated for sqlflow-gke-cluster.
    NAME                 LOCATION       MASTER_VERSION  MASTER_IP    MACHINE_TYPE   NODE_VERSION   NUM_NODES  STATUS
    sqlflow-gke-cluster  us-central1-a  1.12.8-gke.10   108.50.81.7  n1-standard-1  1.12.8-gke.10  3          RUNNING
    ```

    This command will create the Kubernetes clusters with the default version
    documented in the [GKE Versioning and Upgrades](https://cloud.google.com/kubernetes-engine/docs/how-to/creating-a-cluster).

    Note that after this, gcloud will automatically add the proper
    configurations for kubectl CLI. You can verify that kubectl is configured properly by running
    ``` bash
    > kubectl config current-context
    ```
    It should show a cluster name like 'gke_${YOUR_GCP_PROJECT_ID}'-central1-a_sqlflow-gke-cluster


## Deploy SQLFlow Services

You can refer to the [Running SQLFlow on Kubernetes](kubernetes.md)
tutorial for a more comprehensive guide on deploying SQLFlow on Kubernetes
cluster.

In this tutorial, the difference is that we are relying on an existing MySQL
instance (hosted on CloudSQL service). We will show how to deploy a SQLFlow gRPC
server and Jupyter Notebook server.

1. Write demo data to our CloudSQL MySQL instance.
   - Modify doc/k8s/sqlflow-populate-demo-dataset.yaml to replace ${SQLFLOW_MYSQL_HOST}
     with the created CloudSQL MySQL instance private ip address.
   - Launch a one-off job to populate our CloudSQL MySQL instance with demo data.
    ``` bash
    > kubectl apply -f doc/k8s/sqlflow-populate-demo-dataset.yaml
    ```

1. Deploy the SQLFlow service/deployment on Kubernetes
   This is similar to the **All-in-One** example in the [Running SQLFlow on Kubernetes](kubernetes.md),
   except for that only SQLFlow server and Jupyter notebook server are being
   deployed.
   - Modify doc/k8s/sqlflow-all-in-one-without-mysql.yaml to replace ${SQLFLOW_MYSQL_HOST}
     with the created CloudSQL MySQL instance private ip address.
   - Launch a one-off job to populate our CloudSQL MySQL instance with demo data.
    ``` bash
    > kubectl apply -f doc/k8s/sqlflow-all-in-one-without-mysql.yaml
    ```

1. Testing your SQLFlow setup
    You can find a Pod on Kubernetes which name is `sqlflow-all-in-one-without-mysql-<POD-ID>`:
    ``` bash
    > kubectl get pods
    NAME                                                READY   STATUS      RESTARTS   AGE
    sqlflow-all-in-one-without-mysql-856c8bc597-8c8zt   2/2     Running     0          4m5s
    sqlflow-populate-demo-dataset-8qdtd                 0/1     Completed   0          9m30s
    ```

## Run Your Query in SQLFlow

1. Get the external ip address for the sqlflow service
    ``` bash
    > kubectl get services
    NAME                      TYPE           CLUSTER-IP      EXTERNAL-IP     PORT(S)          AGE
    sqlflow-service           LoadBalancer   10.249.11.123   34.65.192.101   8888:32761/TCP   24h
    ```

1. Open a web browser and go to '<EXTERNAL-IP>:8888', you can find the [SQLFlow example](../tutorial/iris-dnn.md) in the Jupyter notebook file lists.

You can refer to the [Running SQLFlow on Kubernetes](kubernetes.md)
tutorial for a more comprehensive guide on deploying SQLFlow on Kubernetes
cluster and how to deploy JupyterHub.


## Clean Up All Resources

You should delete all the GCP resources created in this tutorial (including the
CloudSQL MySQL instance, GKE cluster and VPC) to avoid paying unnecessary bills.
