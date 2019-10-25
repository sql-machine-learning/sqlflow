# Submit Argo Workflow from SQLFlow Container

This document demonstrates how to setup Minikube on your Mac and submit argo workflow from a SQLFlow container.


| command  | version |
|----------|---------|
| minikube | v1.4.0  |
| kubectl  | v1.16   |
| argo     | v2.3.0  |

## Step by Step

**On Host**

1. Install Minikube following this [guide](https://kubernetes.io/docs/tasks/tools/install-minikube/).
1. Start Minikube 
   ```
   minikube start --cpus 2 --memory 4000
   ```
1. Start SQLFlow Docker container. Please be aware that we mount the host `$HOME` as container `/root`, so that the `$HOME/.kube/`(configured by Minikube) can be access inside the container.
   ```
   docker run --rm --net=host -it -v $GOPATH:/go -v $HOME:/root -w /go/src/sqlflow.org/sqlflow sqlflow:latest bash
   ```

**In Container**

1. One more step for sharing the `$HOME/.kube/`. The credential in `$HOME/.kube/config` is referred by absolute path, e.g. `certificate-authority: /Users/yang.y/.minikube/ca.crt`. So we need to create a symbolic link mapping from `/User/yang.y` to `/root`. Please substitute `yang.y` to your user name and type the following command.
   ```
   mkdir /Users && ln -s /root /Users/yang.y
   ```
1. Verify you have access to the Minikube cluster.
   ```
   $ kubectl get namespaces
   NAME              STATUS   AGE
   default           Active   23h
   kube-node-lease   Active   23h
   kube-public       Active   23h
   kube-system       Active   23h
   ```
1. Install the controller and UI.
   ```
   kubectl create namespace argo
   kubectl apply -n argo -f https://raw.githubusercontent.com/argoproj/argo/stable/manifests/install.yaml
   ```
1. Grant admin privileges to the 'default' service account in the namespace 'default', so that the service account can run workflow.
   ```
   kubectl create rolebinding default-admin --clusterrole=admin --serviceaccount=default:default
   ```
1. Install Argo. Skip this step if Argo has been installed.
   ```
   curl -sSL -o /usr/local/bin/argo https://github.com/argoproj/argo/releases/download/v2.3.0/argo-linux-amd64
   chmod +x /usr/local/bin/argo
   ```
1. Run example workflow.
   ```
   argo submit --watch https://raw.githubusercontent.com/argoproj/argo/master/examples/hello-world.yaml
   argo submit --watch https://raw.githubusercontent.com/argoproj/argo/master/examples/coinflip.yaml
   argo submit --watch https://raw.githubusercontent.com/argoproj/argo/master/examples/loops-maps.yaml
   argo list
   argo get xxx-workflow-name-xxx
   argo logs xxx-pod-name-xxx #from get command above
   ```

## Appendix

1. Argo official demo: https://github.com/argoproj/argo/blob/master/demo.md
1. Minikube installation guide: https://kubernetes.io/docs/tasks/tools/install-minikube/
