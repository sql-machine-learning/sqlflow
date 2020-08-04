# A Slim TensorFlow Step Image

This image is used when submitting Argo workflows to run TensorFlow/PAI TensorFlow jobs. To build this image, you should follow the below steps:

1. Go to SQLFlow root directory
    ```bash
    cd $(git rev-parse --show-toplevel)
    ```
1. Build the `sqlflow:dev` docker image as described [here](../dev/README.md), only needed when you haven't done it.  For short, you can just execute below command:
    ```bash
    docker build -t sqlflow:dev -f docker/dev/Dockerfile .
    ```
1. Run `sqlflow:dev` image to build SQLFlow project.  This process will generate a directory called `build` under current directory which contains all the binaries we need.
    ```bash
    docker run --rm -it \
    -v $GOPATH:/root/go \
    -v $HOME/.m2:/root/.m2 \
    -v $HOME/.cache:/root/.cache \
    -v $PWD:/work -w /work \
    sqlflow:dev
    ```
1. Run below command to build the `sqlflow:step` docker image.  This command will package previously built binaries into the result image.
    ```bash
    docker build -t sqlflow:step -f docker/step/Dockerfile .
    ```

To use it in SQLFlow, specify the environment variable `export SQLFLOW_WORKFLOW_STEP_IMAGE=sqlflow:step` when you start `sqlflow server`, or set the image in a Kubernetes config file, you can see an example [here](https://github.com/sql-machine-learning/sqlflow/blob/f5dc0209fe1bd71c443e82ebb4f6981b06e33542/doc/run/k8s/install-sqlflow.yaml#L16).

