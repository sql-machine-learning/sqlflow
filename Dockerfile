FROM python:3.7

# TODO(yi): Currently, SQLFlow core library runs the generated
# TensorFlow program locally, so we need Python, TensorFlow, and MySQL
# connector package installed locally.  In the future, SQLFlow core
# library should fire TensorFlow jobs to Kubernetes clusters, and this
# Dockerfile should be FROM scratch.
RUN pip install --upgrade pip
RUN pip install tensorflow
RUN pip install mysql-connector-python

# sqlflow provides a python client and ipython magic command
RUN pip3 install jupyter
RUN pip3 install sqlflow
# load %%sqlflow by default
# https://stackoverflow.com/a/32683001
ENV IPYTHON_STARTUP /root/.ipython/profile_default/startup/
RUN mkdir -p $IPYTHON_STARTUP
RUN echo 'get_ipython().magic(u"%reload_ext sqlflow.magic")' >> $IPYTHON_STARTUP/00-first.py
RUN echo 'get_ipython().magic(u"%autoreload 2")' >> $IPYTHON_STARTUP/00-first.py

ADD demo /usr/bin/demo
ADD sqlflowserver /usr/bin/sqlflowserver

CMD ["/usr/bin/demo"]
