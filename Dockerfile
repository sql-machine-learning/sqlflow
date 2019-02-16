FROM python:3.7

# TODO(yi): Currently, SQLFlow core library runs the generated
# TensorFlow program locally, so we need Python, TensorFlow, and MySQL
# connector package installed locally.  In the future, SQLFlow core
# library should fire TensorFlow jobs to Kubernetes clusters, and this
# Dockerfile should be FROM scratch.
RUN pip install --upgrade pip
RUN pip install tensorflow
RUN pip install mysql-connector-python

# TODO(yi): Currently, SQLFlow demo doesn't call SQLFlow server, but
# calls SQLFlow core library locally.  We should make it call SQLFlow
# server ASAP.
ADD demo /usr/bin/demo
ADD sqlflowserver /usr/bin/sqlflowserver

CMD ["/usr/bin/demo"]
