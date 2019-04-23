FROM ubuntu:16.04

RUN apt-get update
RUN apt-get install -y python3-pip
RUN pip3 install --upgrade pip
RUN pip3 install tensorflow mysql-connector-python jupyter sqlflow
# Fix jupyter server "connecting to kernel" problem
# https://github.com/jupyter/notebook/issues/2664#issuecomment-468954423
RUN pip3 install tornado==4.5.3

# Load sqlflow Jupyter magic command automatically. c.f. https://stackoverflow.com/a/32683001.
ENV IPYTHON_STARTUP /root/.ipython/profile_default/startup/
RUN mkdir -p $IPYTHON_STARTUP
RUN echo 'get_ipython().magic(u"%reload_ext sqlflow.magic")' >> $IPYTHON_STARTUP/00-first.py
RUN echo 'get_ipython().magic(u"%autoreload 2")' >> $IPYTHON_STARTUP/00-first.py

ADD demo /usr/bin/demo
ADD sqlflowserver /usr/bin/sqlflowserver

# Make python3 callable through python
RUN ln -s /usr/bin/python3 /usr/bin/python
RUN chmod +x /usr/bin/python

CMD ["/usr/bin/demo"]
