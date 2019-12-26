# A Slim Tensorflow Step Image

This image is used when submitting Argo workflows to run Tensorflow/PAI Tensorflow jobs. To build this image, you should follow the below steps:

1. Build the `repl` command binary and copy it under this directory.
2. Copy `python/sqlflow_submitter` directory to this directory.
3. Build and copy the Java parser jar file under this directory.

You can checkout `Dockerfile` to find out how to build above components. After above steps, we are expecting to have below files under current directory before we can run `docker build`:

- repl
- parser-1.0-SNAPSHOT-jar-with-dependencies.jar
- sqlflow_submitter/

Then run `docker build -t sqlflow_tensorflow_step:slim .`, and push this image to some registry that your Kubernetes cluster can reach. To use it in SQLFlow, specify the environment variable `export SQLFLOW_WORKFLOW_STEP_IMAG=sqlflow_tensorflow_step:slim` to use this image as step image.

