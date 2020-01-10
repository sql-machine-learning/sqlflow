# A Slim Tensorflow Step Image

This image is used when submitting Argo workflows to run Tensorflow/PAI Tensorflow jobs. To build this image, you should follow the below steps:

1. Run `docker run --rm -it -v $PWD:/opt/output sqlflow/sqlflow cp -r /usr/local/bin/repl /opt/sqlflow-parser/parser-1.0-SNAPSHOT-jar-with-dependencies.jar /opt/output` to copy latest `repl` and parser jar file.
2. Copy `python/sqlflow_submitter` directory to this directory.

After above steps, we are expecting to have below files under the current directory before we can run `docker build`:

- repl
- parser-1.0-SNAPSHOT-jar-with-dependencies.jar
- sqlflow_submitter/

Then run `docker build -t sqlflow_tensorflow_step:slim .`, and push this image to some registry that your Kubernetes cluster can reach. To use it in SQLFlow, specify the environment variable `export SQLFLOW_WORKFLOW_STEP_IMAGE=sqlflow_tensorflow_step:slim` to use this image as step image.

