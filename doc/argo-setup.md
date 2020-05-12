# Submit Argo Workflow from SQLFlow Container

In this document, we explain how to submit jobs from a SQLFlow server container to a Kubernetes cluster.  We use Minikube on a Mac, but you can use Kubernetes clusters on public cloud services as well.  The jobs we submit in this document are Argo workflows.

Please be aware that, in practice, the SQLFlow container might be running on the Kubernetes cluster as Argo workflows, but not in a separate container. And, it is the submitter program running in the SQLFlow server container who submits Argo workflows by calling the Kubernetes API other than the argo command.  It is known as **Kubernetes-native** to call Kubernetes APIs from a container running on the Kubernetes cluster. For how to implement Kubernetes-native calls, please refer to the ElasticDL master program as an example.

## On the Mac

1. Install [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/).
1. Start Minikube
   ```bash
   minikube start --cpus 2 --memory 4000
   ```
1. Start a SQLFlow Docker container.
   ```bash
   docker run --rm --net=host -it -v $GOPATH:/go -v $HOME:/root -w /go/src/sqlflow.org/sqlflow sqlflow/sqlflow:ci bash
   ```
   We use `-v $HOME:/root` to mount the home directory on the host, `$HOME`, to the home directory in the container, `/root`, so we can access the Minikube cluster configuration files in `$HOME/.kube/` from within the container.

## In the SQLFlow Container

1. One more step for sharing the `$HOME/.kube/`. The credential in `$HOME/.kube/config` is referred to by absolute path, e.g. `certificate-authority: /Users/yang.y/.minikube/ca.crt`. So we need to create a symbolic link mapping `/User/yang.y` to `/root`. Please substitute `yang.y` to your user name and type the following command.
   ```
   mkdir /Users && ln -s /root /Users/yang.y
   ```
1. Verify you have access to the Minikube cluster by typing the following command in the container.
   ```
   $ kubectl get namespaces
   NAME              STATUS   AGE
   default           Active   23h
   kube-node-lease   Active   23h
   kube-public       Active   23h
   kube-system       Active   23h
   ```
1. Install the Argo controller and UI.
   ```bash
   kubectl create namespace argo
   kubectl apply -n argo -f https://raw.githubusercontent.com/argoproj/argo/stable/manifests/install.yaml
   ```
1. Grant admin privileges to the 'default' service account in the namespace `default`, so that the service account can run workflow.
   ```
   kubectl create rolebinding default-admin --clusterrole=admin --serviceaccount=default:default
   ```
1. Run example workflow. Please be aware that the generated workflow name may differ between runs.
   ```
   $ kubectl create -f https://raw.githubusercontent.com/argoproj/argo/master/examples/hello-world.yaml
   workflow.argoproj.io/hello-world-hskf4 created

   $ kubectl logs hello-world-hskf4 main
      _____________
     < hello world >
      -------------
         \
          \
           \
                         ##        .
                   ## ## ##       ==
                ## ## ## ##      ===
            /""""""""""""""""___/ ===
       ~~~ {~~ ~~~~ ~~~ ~~~~ ~~ ~ /  ===- ~~~
            \______ o          __/
             \    \        __/
               \____\______/
   ```

## Appendix

1. Argo official demo: https://github.com/argoproj/argo/blob/master/demo.md
1. Minikube installation guide: https://kubernetes.io/docs/tasks/tools/install-minikube/
